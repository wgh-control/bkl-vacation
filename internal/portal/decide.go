package portal

import (
	"context"

	"github.com/wgh-control/bkl-vacation/internal/identity"
	"github.com/wgh-control/bkl-vacation/internal/leave"
	"github.com/wgh-control/bkl-vacation/internal/model"
	"github.com/wgh-control/bkl-vacation/internal/store"
)

// ListApprovals is the manager queue: direct reports waiting on the manager.
func (s *Service) ListApprovals(ctx context.Context, p identity.Principal) ([]QueueItem, error) {
	var out []QueueItem
	err := s.tx(ctx, func(q store.Queries) error {
		emp, err := s.require(ctx, q, p, false)
		if err != nil {
			return err
		}
		reqs, err := q.ListRequests(ctx, model.RequestFilter{ManagerID: emp.ID, Statuses: []string{string(model.StatusPendingManager)}})
		if err != nil {
			return err
		}
		out, err = s.queue(ctx, q, reqs, "manager")
		return err
	})
	return out, err
}

// Decide records a manager or HR approval. One pending step at a time.
func (s *Service) Decide(ctx context.Context, p identity.Principal, id string, in DecisionInput) (RequestView, error) {
	decision := in.Decision
	if decision != "approve" && decision != "reject" {
		return RequestView{}, code("validation", "decision must be approve or reject", 400)
	}
	if err := checkComment(in.Comment); err != nil {
		return RequestView{}, err
	}
	var out RequestView
	err := s.tx(ctx, func(q store.Queries) error {
		actor, err := s.require(ctx, q, p, true)
		if err != nil {
			return err
		}
		req, err := q.GetRequest(ctx, id)
		if err != nil {
			return missing(err)
		}
		owner, err := q.GetEmployee(ctx, req.EmployeeID)
		if err != nil {
			return missing(err)
		}
		actedAsManager := false
		if req.Status == model.StatusPendingManager {
			ok, mgr := s.managerOf(ctx, q, owner)
			if ok && actor.ID == mgr.ID {
				if err := s.managerDecision(ctx, q, actor, owner, &req, decision, in.Comment); err != nil {
					return err
				}
				actedAsManager = true
			} else if !ok && p.HasGroup(s.HRGroup) {
				before := req.Status
				req.Status = model.StatusPendingHR
				req.SelfApproval = owner.ManagerID != "" && owner.ManagerID == owner.ID
				req.ManagerVacant = !req.SelfApproval
				req.UpdatedAt = s.now()
				if err := q.UpdateRequest(ctx, req); err != nil {
					return err
				}
				if err := s.audit(ctx, q, actor.ID, req.ID, owner.ID, "manager_unavailable", string(before), string(req.Status)); err != nil {
					return err
				}
			} else {
				return code("forbidden", "only the current manager can decide this request", 403)
			}
		}
		handled := actedAsManager
		if !handled && req.Status == model.StatusPendingHR {
			if !p.HasGroup(s.HRGroup) {
				return code("forbidden", "HR group required", 403)
			}
			if err := s.hrDecision(ctx, q, actor, owner, &req, decision, in.Comment); err != nil {
				return err
			}
			handled = true
		}
		if !handled {
			return code("invalid_status", "the request is not waiting for a decision", 409)
		}
		aud := "manager"
		if p.HasGroup(s.HRGroup) {
			aud = "hr"
		}
		if actor.ID == owner.ID {
			aud = "self"
		}
		out, err = s.viewRequest(ctx, q, req, aud)
		return err
	})
	return out, err
}

func (s *Service) managerDecision(ctx context.Context, q store.Queries, actor, owner model.Employee, req *model.Request, decision, comment string) error {
	if decision == "reject" && managerMustGrant(*req) {
		return code("reason_mandatory", "the manager cannot refuse this day off", 400)
	}
	if err := s.writeStep(ctx, q, req.ID, "manager", actor, decision, comment); err != nil {
		return err
	}
	before := req.Status
	if decision == "reject" {
		req.Status = model.StatusRejected
	} else {
		req.Status = model.StatusPendingHR
	}
	req.UpdatedAt = s.now()
	if err := q.UpdateRequest(ctx, *req); err != nil {
		return err
	}
	return s.audit(ctx, q, actor.ID, req.ID, owner.ID, decision, string(before), string(req.Status))
}

func (s *Service) hrDecision(ctx context.Context, q store.Queries, actor, owner model.Employee, req *model.Request, decision, comment string) error {
	if req.Status != model.StatusPendingHR {
		return code("invalid_status", "HR has nothing to decide", 409)
	}
	if (req.SelfApproval || req.ManagerVacant) && actor.ID == owner.ID {
		return code("self_approval", "another HR person must decide a request that skipped the manager", 403)
	}
	if decision == "reject" {
		if err := s.writeStep(ctx, q, req.ID, "hr", actor, decision, comment); err != nil {
			return err
		}
		before := req.Status
		req.Status = model.StatusRejected
		req.UpdatedAt = s.now()
		if err := q.UpdateRequest(ctx, *req); err != nil {
			return err
		}
		return s.audit(ctx, q, actor.ID, req.ID, owner.ID, "reject", string(before), string(req.Status))
	}
	if req.Type == model.TypeSick {
		if req.CertificateNumber == "" {
			return code("certificate_required", "HR cannot approve a sick request without a certificate number", 400)
		}
		if err := s.applySickOverlap(ctx, q, owner, req); err != nil {
			return err
		}
	}
	if req.Type == model.TypeVacation && req.ReplacesID != "" {
		if err := s.recheckMove(ctx, q, owner, *req); err != nil {
			return err
		}
	}
	if err := s.writeStep(ctx, q, req.ID, "hr", actor, decision, comment); err != nil {
		return err
	}
	before := req.Status
	now := s.now()
	req.NeedsConfirmation = req.Type == model.TypeVacation || (req.Type == model.TypeDayOff && leave.DaysUntil(now, req.StartsOn) >= 5)
	if req.NeedsConfirmation && leave.ConfirmationOpen(now, req.StartsOn) {
		req.Status = model.StatusAwaitingConfirmation
	} else {
		req.Status = model.StatusScheduled
	}
	req.ConfirmedAt = nil
	req.AcknowledgedAt = nil
	req.ProcessedAt = nil
	req.ProcessedBy = ""
	req.UpdatedAt = now
	if err := q.UpdateRequest(ctx, *req); err != nil {
		return err
	}
	if req.ReplacesID != "" {
		old, err := q.GetRequest(ctx, req.ReplacesID)
		if err != nil {
			return err
		}
		prev := old.Status
		old.Status = model.StatusCancelled
		old.CancelReason = model.CancelRescheduled
		old.UpdatedAt = now
		if err := q.UpdateRequest(ctx, old); err != nil {
			return err
		}
		if err := s.audit(ctx, q, actor.ID, old.ID, owner.ID, "rescheduled", string(prev), string(old.Status)); err != nil {
			return err
		}
	}
	if err := s.ensureRequestNotices(ctx, q, *req, owner); err != nil {
		return err
	}
	return s.audit(ctx, q, actor.ID, req.ID, owner.ID, "approve", string(before), string(req.Status))
}

func (s *Service) writeStep(ctx context.Context, q store.Queries, requestID, role string, actor model.Employee, decision, comment string) error {
	steps, err := q.ListSteps(ctx, requestID)
	if err != nil {
		return err
	}
	now := s.now()
	for _, st := range steps {
		if st.Role == role && st.Decision == "" {
			st.Decision = decision
			st.Comment = comment
			st.ActorID = actor.ID
			st.DecidedAt = &now
			if role == "manager" {
				st.ExpectedEmployeeID = actor.ID
			}
			return q.UpdateStep(ctx, st)
		}
	}
	seq := len(steps) + 1
	return q.InsertStep(ctx, model.Step{
		ID: s.id(), RequestID: requestID, Sequence: seq, Role: role,
		ExpectedEmployeeID: actor.ID, Decision: decision, Comment: comment, DecidedAt: &now, ActorID: actor.ID,
	})
}

func managerMustGrant(r model.Request) bool {
	spec, ok := lookupReason(r.ReasonCode)
	if !ok || !spec.Mandatory {
		return false
	}
	if spec.MaxDays > 0 && leave.DaysInclusive(r.StartsOn, r.EndsOn) > spec.MaxDays {
		return false
	}
	return true
}

func (s *Service) recheckMove(ctx context.Context, q store.Queries, owner model.Employee, req model.Request) error {
	year, err := s.yearFor(ctx, q, owner, req.StartsOn)
	if err != nil {
		return err
	}
	if err := q.LockLeaveYear(ctx, year.ID); err != nil {
		return err
	}
	reqs, err := q.ListRequests(ctx, model.RequestFilter{EmployeeID: owner.ID})
	if err != nil {
		return err
	}
	hol, err := s.holidayFn(ctx, q, req.StartsOn.AddDate(0, 0, -2), req.EndsOn.AddDate(0, 0, 40))
	if err != nil {
		return err
	}
	return s.checkVacation(owner, year, reqs, req.StartsOn, req.EndsOn, req.CountedDays, req.ReplacesID, hol, sickRequests(reqs))
}

func (s *Service) applySickOverlap(ctx context.Context, q store.Queries, owner model.Employee, sick *model.Request) error {
	reqs, err := q.ListRequests(ctx, model.RequestFilter{EmployeeID: owner.ID})
	if err != nil {
		return err
	}
	hol, err := s.holidayFn(ctx, q, sick.StartsOn.AddDate(0, 0, -2), sick.EndsOn.AddDate(0, 0, 80))
	if err != nil {
		return err
	}
	var hits []model.Request
	for _, r := range reqs {
		if r.Type == model.TypeVacation && approvedStatus(r.Status) && leave.Overlaps(r.StartsOn, r.EndsOn, sick.StartsOn, sick.EndsOn) {
			hits = append(hits, r)
		}
	}
	if len(hits) == 0 {
		return nil
	}
	if sick.OverlapChoice != "extend" && sick.OverlapChoice != "postpone" {
		return code("overlap_choice_required", "choose extend or postpone for the overlapping vacation", 400)
	}
	now := s.now()
	for _, v := range hits {
		base := vacationDates(v, hol, nil)
		overlap := leave.IntersectDates(base, leave.CalendarDays(sick.StartsOn, sick.EndsOn))
		if len(overlap) == 0 {
			continue
		}
		if sick.OverlapChoice == "extend" {
			v.EndsOn = leave.ExtendEnd(v.EndsOn, len(overlap), hol)
			years, err := q.ListLeaveYears(ctx, owner.ID)
			if err != nil {
				return err
			}
			for _, y := range years {
				if y.ID == v.LeaveYearID && v.EndsOn.After(y.EndsOn) {
					return code("crosses_working_year", "the extension would leave the working year", 400)
				}
			}
		}
		marked := markApproved(reqs, sick.ID)
		v.CountedDays = requestDays(v, hol, sickRequests(marked))
		v.UpdatedAt = now
		if err := q.UpdateRequest(ctx, v); err != nil {
			return err
		}
	}
	if sick.OverlapChoice == "postpone" {
		reloaded, err := q.ListRequests(ctx, model.RequestFilter{EmployeeID: owner.ID})
		if err != nil {
			return err
		}
		reloaded = markApproved(reloaded, sick.ID)
		hol, err = s.holidayFn(ctx, q, sick.StartsOn.AddDate(-1, 0, 0), sick.EndsOn.AddDate(1, 0, 0))
		if err != nil {
			return err
		}
		years, err := q.ListLeaveYears(ctx, owner.ID)
		if err != nil {
			return err
		}
		for _, y := range years {
			st := s.statsForYear(reloaded, y, hol, sickRequests(reloaded), "")
			grant := y.GrantedDays + y.ExtraDays
			if !leave.StreakPossible(st.parts, grant, st.used+st.reserved) {
				return code("streak_impossible", "postponing would leave no 14-day vacation part", 400)
			}
		}
	}
	return nil
}

func markApproved(reqs []model.Request, id string) []model.Request {
	out := make([]model.Request, len(reqs))
	copy(out, reqs)
	for i := range out {
		if out[i].ID == id {
			out[i].Status = model.StatusScheduled
		}
	}
	return out
}

func (s *Service) queue(ctx context.Context, q store.Queries, reqs []model.Request, aud string) ([]QueueItem, error) {
	var out []QueueItem
	for _, r := range reqs {
		view, err := s.viewRequest(ctx, q, r, aud)
		if err != nil {
			return nil, err
		}
		owner, err := q.GetEmployee(ctx, r.EmployeeID)
		if err != nil {
			return nil, err
		}
		item := QueueItem{
			RequestView: view, EmployeeName: owner.DisplayName, EmployeeNumber: owner.EmployeeNumber, Position: owner.Position,
		}
		if owner.DepartmentID != "" {
			if u, err := q.GetOrgUnit(ctx, owner.DepartmentID); err == nil {
				item.Department = u.Name
			}
		}
		steps, err := q.ListSteps(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		for _, st := range steps {
			if st.Role == "manager" && st.Decision != "" {
				item.ManagerDecision = &DecisionView{Decision: st.Decision, Comment: st.Comment, At: stamp(st.DecidedAt), ActorID: st.ActorID}
			}
		}
		out = append(out, item)
	}
	if out == nil {
		out = []QueueItem{}
	}
	return out, nil
}
