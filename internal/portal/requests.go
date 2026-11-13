package portal

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/wgh-control/bkl-vacation/internal/identity"
	"github.com/wgh-control/bkl-vacation/internal/leave"
	"github.com/wgh-control/bkl-vacation/internal/model"
	"github.com/wgh-control/bkl-vacation/internal/store"
)

// CreateRequest submits a vacation, day off, or sick day for the signed-in employee.
// HR uses the same call. A vacation may be planned for the next calendar year.
func (s *Service) CreateRequest(ctx context.Context, p identity.Principal, in CreateInput) (RequestView, error) {
	var out RequestView
	err := s.tx(ctx, func(q store.Queries) error {
		emp, err := s.require(ctx, q, p, true)
		if err != nil {
			return err
		}
		req, err := s.build(ctx, q, emp, in, "")
		if err != nil {
			return err
		}
		steps := s.routeNew(ctx, q, emp, &req)
		if err := q.InsertRequest(ctx, req); err != nil {
			return err
		}
		for _, st := range steps {
			if err := q.InsertStep(ctx, st); err != nil {
				return err
			}
		}
		if err := s.audit(ctx, q, emp.ID, req.ID, emp.ID, "create", "", string(req.Status)); err != nil {
			return err
		}
		if err := s.queueSubmitted(ctx, q, emp, req); err != nil {
			return err
		}
		out, err = s.viewRequest(ctx, q, req, "self")
		return err
	})
	return out, err
}

func (s *Service) build(ctx context.Context, q store.Queries, emp model.Employee, in CreateInput, releaseID string) (model.Request, error) {
	typ := model.RequestType(in.Type)
	switch typ {
	case model.TypeVacation, model.TypeDayOff, model.TypeSick:
	default:
		return model.Request{}, code("validation", "type must be vacation, day_off, or sick", 400)
	}
	if in.OverlapChoice != "" && in.OverlapChoice != "extend" && in.OverlapChoice != "postpone" {
		return model.Request{}, code("validation", "overlap_choice must be extend or postpone", 400)
	}
	start, end, err := s.resolveSpan(ctx, q, typ, in)
	if err != nil {
		return model.Request{}, err
	}
	if !leave.PlanningHorizon(s.now(), start, end, typ == model.TypeSick) {
		return model.Request{}, code("planning_horizon", "new requests can be planned for this calendar year or the next", 400)
	}
	if typ == model.TypeDayOff {
		if _, ok := lookupReason(in.ReasonCode); !ok {
			return model.Request{}, code("validation", "unknown day-off reason", 400)
		}
	}
	hol, err := s.holidayFn(ctx, q, start.AddDate(0, 0, -2), end.AddDate(0, 0, 80))
	if err != nil {
		return model.Request{}, err
	}
	var days int
	if typ == model.TypeVacation {
		days = len(leave.CountedVacationDays(start, end, hol))
	} else {
		days = leave.DaysInclusive(start, end)
	}
	if days <= 0 {
		return model.Request{}, code("empty_range", "the range has no counted days", 400)
	}
	year, err := s.yearFor(ctx, q, emp, start)
	if err != nil {
		return model.Request{}, err
	}
	if leave.CrossesYear(start, end, year.StartsOn, year.EndsOn) {
		return model.Request{}, code("crosses_working_year", "a request must stay inside one working year", 400)
	}
	if err := q.LockLeaveYear(ctx, year.ID); err != nil {
		return model.Request{}, err
	}
	reqs, err := q.ListRequests(ctx, model.RequestFilter{EmployeeID: emp.ID})
	if err != nil {
		return model.Request{}, err
	}
	for _, existing := range reqs {
		if rangesConflict(existing, start, end, typ, releaseID) {
			return model.Request{}, code("overlap", "the dates overlap another request", 400)
		}
	}
	sicks := sickRequests(reqs)
	if typ == model.TypeVacation {
		if err := s.checkVacation(emp, year, reqs, start, end, days, releaseID, hol, sicks); err != nil {
			return model.Request{}, err
		}
	}
	now := s.now()
	return model.Request{
		ID: s.id(), EmployeeID: emp.ID, LeaveYearID: year.ID, Type: typ,
		StartsOn: start, EndsOn: end, CountedDays: days, ReasonCode: in.ReasonCode,
		CertificateNumber: strings.TrimSpace(in.CertificateNumber), OverlapChoice: in.OverlapChoice,
		ReplacesID: releaseID, PayByWarning: typ == model.TypeVacation && leave.PayByMissed(now, start),
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (s *Service) resolveSpan(ctx context.Context, q store.Queries, typ model.RequestType, in CreateInput) (start, end time.Time, err error) {
	if strings.TrimSpace(in.StartsOn) == "" {
		return time.Time{}, time.Time{}, code("validation", "starts_on is required", 400)
	}
	start, err = leave.ParseDate(in.StartsOn)
	if err != nil {
		return time.Time{}, time.Time{}, code("validation", "starts_on must be YYYY-MM-DD", 400)
	}
	if strings.TrimSpace(in.EndsOn) != "" {
		end, err = leave.ParseDate(in.EndsOn)
		if err != nil {
			return time.Time{}, time.Time{}, code("validation", "ends_on must be YYYY-MM-DD", 400)
		}
		if end.Before(start) {
			return time.Time{}, time.Time{}, code("validation", "ends_on is before starts_on", 400)
		}
		return start, end, nil
	}
	if typ == model.TypeVacation && in.CountedDays > 0 {
		hol, err := s.holidayFn(ctx, q, start, start.AddDate(0, 0, 400))
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		var ok bool
		end, ok = leave.EndForCountedDays(start, in.CountedDays, hol)
		if !ok {
			return time.Time{}, time.Time{}, code("validation", "counted_days could not be placed", 400)
		}
		return start, end, nil
	}
	return time.Time{}, time.Time{}, code("validation", "ends_on is required", 400)
}

func (s *Service) routeNew(ctx context.Context, q store.Queries, emp model.Employee, req *model.Request) []model.Step {
	ok, mgr := s.managerOf(ctx, q, emp)
	if ok {
		req.Status = model.StatusPendingManager
		return []model.Step{
			{ID: s.id(), RequestID: req.ID, Sequence: 1, Role: "manager", ExpectedEmployeeID: mgr.ID},
			{ID: s.id(), RequestID: req.ID, Sequence: 2, Role: "hr"},
		}
	}
	req.Status = model.StatusPendingHR
	req.SelfApproval = emp.ManagerID != "" && emp.ManagerID == emp.ID
	req.ManagerVacant = !req.SelfApproval
	return []model.Step{{ID: s.id(), RequestID: req.ID, Sequence: 1, Role: "hr"}}
}

func (s *Service) managerOf(ctx context.Context, q store.Queries, emp model.Employee) (bool, model.Employee) {
	if emp.ManagerID == "" || emp.ManagerID == emp.ID {
		return false, model.Employee{}
	}
	mgr, err := q.GetEmployee(ctx, emp.ManagerID)
	if err != nil || !mgr.Active {
		return false, model.Employee{}
	}
	return true, mgr
}

// ListMine returns the caller's own requests.
func (s *Service) ListMine(ctx context.Context, p identity.Principal) ([]RequestView, error) {
	var out []RequestView
	err := s.tx(ctx, func(q store.Queries) error {
		emp, err := s.require(ctx, q, p, false)
		if err != nil {
			return err
		}
		reqs, err := q.ListRequests(ctx, model.RequestFilter{EmployeeID: emp.ID})
		if err != nil {
			return err
		}
		for i := range reqs {
			if err := s.refreshConfirmation(ctx, q, &reqs[i]); err != nil {
				return err
			}
		}
		out, err = s.views(ctx, q, reqs, "self")
		return err
	})
	return out, err
}

// GetRequest returns one request to its owner, their manager, or HR.
func (s *Service) GetRequest(ctx context.Context, p identity.Principal, id string) (RequestView, error) {
	var out RequestView
	err := s.tx(ctx, func(q store.Queries) error {
		emp, err := s.require(ctx, q, p, false)
		if err != nil {
			return err
		}
		req, err := q.GetRequest(ctx, id)
		if err != nil {
			return missing(err)
		}
		if err := s.refreshConfirmation(ctx, q, &req); err != nil {
			return err
		}
		aud, err := s.audience(ctx, q, emp, p, req)
		if err != nil {
			return err
		}
		out, err = s.viewRequest(ctx, q, req, aud)
		return err
	})
	return out, err
}

func (s *Service) audience(ctx context.Context, q store.Queries, emp model.Employee, p identity.Principal, req model.Request) (string, error) {
	if req.EmployeeID == emp.ID {
		return "self", nil
	}
	if p.HasGroup(s.HRGroup) {
		return "hr", nil
	}
	owner, err := q.GetEmployee(ctx, req.EmployeeID)
	if err != nil {
		return "", missing(err)
	}
	if owner.ManagerID == emp.ID {
		return "manager", nil
	}
	return "", code("forbidden", "this request belongs to someone else", 403)
}

// Cancel withdraws a request until HR has marked it processed.
func (s *Service) Cancel(ctx context.Context, p identity.Principal, id string) (RequestView, error) {
	var out RequestView
	err := s.tx(ctx, func(q store.Queries) error {
		emp, err := s.require(ctx, q, p, true)
		if err != nil {
			return err
		}
		req, err := q.GetRequest(ctx, id)
		if err != nil {
			return missing(err)
		}
		if req.EmployeeID != emp.ID {
			return code("forbidden", "only the owner can cancel", 403)
		}
		if req.Status == model.StatusProcessed {
			return code("cannot_cancel", "a processed request is changed by a reschedule", 409)
		}
		if req.Status == model.StatusRejected || req.Status == model.StatusCancelled {
			return code("invalid_status", "the request is already closed", 409)
		}
		if req.SuccessorID != "" {
			if succ, err := q.GetRequest(ctx, req.SuccessorID); err == nil && reserves(succ.Status) {
				before := succ.Status
				succ.Status = model.StatusCancelled
				succ.UpdatedAt = s.now()
				if err := q.UpdateRequest(ctx, succ); err != nil {
					return err
				}
				if err := s.audit(ctx, q, emp.ID, succ.ID, emp.ID, "cancel", string(before), string(succ.Status)); err != nil {
					return err
				}
			}
		}
		before := req.Status
		req.Status = model.StatusCancelled
		req.UpdatedAt = s.now()
		if err := q.UpdateRequest(ctx, req); err != nil {
			return err
		}
		if err := s.audit(ctx, q, emp.ID, req.ID, emp.ID, "cancel", string(before), string(req.Status)); err != nil {
			return err
		}
		out, err = s.viewRequest(ctx, q, req, "self")
		return err
	})
	return out, err
}

// Reschedule proposes new dates. The old dates stay reserved until manager and HR approve.
func (s *Service) Reschedule(ctx context.Context, p identity.Principal, id string, in CreateInput) (RequestView, error) {
	var out RequestView
	err := s.tx(ctx, func(q store.Queries) error {
		emp, err := s.require(ctx, q, p, true)
		if err != nil {
			return err
		}
		old, err := q.GetRequest(ctx, id)
		if err != nil {
			return missing(err)
		}
		if old.EmployeeID != emp.ID {
			return code("forbidden", "only the owner can reschedule", 403)
		}
		if old.Status == model.StatusRejected || old.Status == model.StatusCancelled {
			return code("invalid_status", "a closed request cannot be rescheduled", 409)
		}
		if old.SuccessorID != "" {
			succ, err := q.GetRequest(ctx, old.SuccessorID)
			if err == nil && succ.Status != model.StatusRejected && succ.Status != model.StatusCancelled {
				return code("move_pending", "a reschedule is already waiting", 409)
			}
		}
		in.Type = string(old.Type)
		in.ReasonCode = first(in.ReasonCode, old.ReasonCode)
		in.CertificateNumber = first(in.CertificateNumber, old.CertificateNumber)
		next, err := s.build(ctx, q, emp, in, old.ID)
		if err != nil {
			return err
		}
		steps := s.routeNew(ctx, q, emp, &next)
		if err := q.InsertRequest(ctx, next); err != nil {
			return err
		}
		for _, st := range steps {
			if err := q.InsertStep(ctx, st); err != nil {
				return err
			}
		}
		old.SuccessorID = next.ID
		old.UpdatedAt = s.now()
		if err := q.UpdateRequest(ctx, old); err != nil {
			return err
		}
		if err := s.audit(ctx, q, emp.ID, next.ID, emp.ID, "reschedule", string(old.Status), string(next.Status)); err != nil {
			return err
		}
		if err := s.queueSubmitted(ctx, q, emp, next); err != nil {
			return err
		}
		out, err = s.viewRequest(ctx, q, next, "self")
		return err
	})
	return out, err
}

// Confirm records the company confirmation inside the 3–5 day window, including a late one.
func (s *Service) Confirm(ctx context.Context, p identity.Principal, id string) (RequestView, error) {
	var out RequestView
	err := s.tx(ctx, func(q store.Queries) error {
		emp, err := s.require(ctx, q, p, true)
		if err != nil {
			return err
		}
		req, err := q.GetRequest(ctx, id)
		if err != nil {
			return missing(err)
		}
		if req.EmployeeID != emp.ID {
			return code("forbidden", "only the owner can confirm", 403)
		}
		if !req.NeedsConfirmation {
			return code("invalid_status", "this request does not ask for confirmation", 409)
		}
		if req.ConfirmedAt != nil || req.Status == model.StatusConfirmed || req.Status == model.StatusProcessed {
			return code("invalid_status", "the request is already confirmed", 409)
		}
		if req.Status != model.StatusScheduled && req.Status != model.StatusAwaitingConfirmation {
			return code("invalid_status", "the request is not ready to confirm", 409)
		}
		if !leave.ConfirmationOpen(s.now(), req.StartsOn) {
			return code("confirmation_too_early", "confirmation opens five days before the start", 409)
		}
		before := req.Status
		now := s.now()
		req.Status = model.StatusConfirmed
		req.ConfirmedAt = &now
		req.UpdatedAt = now
		if err := q.UpdateRequest(ctx, req); err != nil {
			return err
		}
		if err := s.ensureRequestNotices(ctx, q, req, emp); err != nil {
			return err
		}
		if err := s.audit(ctx, q, emp.ID, req.ID, emp.ID, "confirm", string(before), string(req.Status)); err != nil {
			return err
		}
		out, err = s.viewRequest(ctx, q, req, "self")
		return err
	})
	return out, err
}

// Acknowledge stores the article 123 notice acknowledgement.
func (s *Service) Acknowledge(ctx context.Context, p identity.Principal, id string) (RequestView, error) {
	var out RequestView
	err := s.tx(ctx, func(q store.Queries) error {
		emp, err := s.require(ctx, q, p, true)
		if err != nil {
			return err
		}
		req, err := q.GetRequest(ctx, id)
		if err != nil {
			return missing(err)
		}
		if req.EmployeeID != emp.ID {
			return code("forbidden", "only the owner can acknowledge", 403)
		}
		if !approvedStatus(req.Status) && req.Status != model.StatusAwaitingConfirmation {
			return code("invalid_status", "the start date is not scheduled yet", 409)
		}
		if req.AcknowledgedAt == nil {
			now := s.now()
			req.AcknowledgedAt = &now
			req.UpdatedAt = now
			if err := q.UpdateRequest(ctx, req); err != nil {
				return err
			}
			if err := s.audit(ctx, q, emp.ID, req.ID, emp.ID, "acknowledge", string(req.Status), string(req.Status)); err != nil {
				return err
			}
		}
		out, err = s.viewRequest(ctx, q, req, "self")
		return err
	})
	return out, err
}

// SetCertificate adds the sick-certificate number before HR acts.
func (s *Service) SetCertificate(ctx context.Context, p identity.Principal, id, number string) (RequestView, error) {
	number = strings.TrimSpace(number)
	if number == "" {
		return RequestView{}, code("validation", "certificate number is empty", 400)
	}
	var out RequestView
	err := s.tx(ctx, func(q store.Queries) error {
		emp, err := s.require(ctx, q, p, true)
		if err != nil {
			return err
		}
		req, err := q.GetRequest(ctx, id)
		if err != nil {
			return missing(err)
		}
		if req.EmployeeID != emp.ID {
			return code("forbidden", "only the owner can set the certificate", 403)
		}
		if req.Type != model.TypeSick {
			return code("validation", "only a sick request has a certificate", 400)
		}
		if !inFlight(req.Status) {
			return code("invalid_status", "HR has already decided", 409)
		}
		req.CertificateNumber = number
		req.UpdatedAt = s.now()
		if err := q.UpdateRequest(ctx, req); err != nil {
			return err
		}
		out, err = s.viewRequest(ctx, q, req, "self")
		return err
	})
	return out, err
}

// SetOverlapChoice records extend or postpone for a sick day that meets a vacation.
func (s *Service) SetOverlapChoice(ctx context.Context, p identity.Principal, id, choice string) (RequestView, error) {
	if choice != "extend" && choice != "postpone" {
		return RequestView{}, code("validation", "overlap_choice must be extend or postpone", 400)
	}
	var out RequestView
	err := s.tx(ctx, func(q store.Queries) error {
		emp, err := s.require(ctx, q, p, true)
		if err != nil {
			return err
		}
		req, err := q.GetRequest(ctx, id)
		if err != nil {
			return missing(err)
		}
		if req.EmployeeID != emp.ID {
			return code("forbidden", "only the owner can choose", 403)
		}
		if req.Type != model.TypeSick || !inFlight(req.Status) {
			return code("invalid_status", "the choice is made on a sick request before HR decides", 409)
		}
		req.OverlapChoice = choice
		req.UpdatedAt = s.now()
		if err := q.UpdateRequest(ctx, req); err != nil {
			return err
		}
		out, err = s.viewRequest(ctx, q, req, "self")
		return err
	})
	return out, err
}

func (s *Service) viewRequest(ctx context.Context, q store.Queries, req model.Request, aud string) (RequestView, error) {
	reqs, err := q.ListRequests(ctx, model.RequestFilter{EmployeeID: req.EmployeeID})
	if err != nil {
		return RequestView{}, err
	}
	hol, err := s.holidayFn(ctx, q, req.StartsOn.AddDate(0, 0, -2), req.EndsOn.AddDate(0, 0, 40))
	if err != nil {
		return RequestView{}, err
	}
	var year *model.LeaveYear
	if req.LeaveYearID != "" {
		years, err := q.ListLeaveYears(ctx, req.EmployeeID)
		if err != nil {
			return RequestView{}, err
		}
		for i := range years {
			if years[i].ID == req.LeaveYearID {
				year = &years[i]
				break
			}
		}
	}
	return s.present(req, aud, hol, sickRequests(reqs), year), nil
}

func (s *Service) views(ctx context.Context, q store.Queries, reqs []model.Request, aud string) ([]RequestView, error) {
	out := make([]RequestView, 0, len(reqs))
	for _, r := range reqs {
		v, err := s.viewRequest(ctx, q, r, aud)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if out == nil {
		out = []RequestView{}
	}
	return out, nil
}

func sickRequests(reqs []model.Request) []model.Request {
	var out []model.Request
	for _, r := range reqs {
		if r.Type == model.TypeSick {
			out = append(out, r)
		}
	}
	return out
}

func missing(err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return code("not_found", "not found", 404)
	}
	return err
}
