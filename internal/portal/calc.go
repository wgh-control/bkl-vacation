package portal

import (
	"context"
	"time"

	"github.com/wgh-control/bkl-vacation/internal/calendar"
	"github.com/wgh-control/bkl-vacation/internal/leave"
	"github.com/wgh-control/bkl-vacation/internal/model"
	"github.com/wgh-control/bkl-vacation/internal/store"
)

func (s *Service) holidayFn(ctx context.Context, q store.Queries, from, to time.Time) (func(time.Time) bool, error) {
	if from.IsZero() {
		from = s.now().AddDate(-1, 0, 0)
	}
	if to.IsZero() || to.Before(from) {
		to = from.AddDate(2, 0, 0)
	}
	days, err := q.ListCalendarDays(ctx, from, to)
	if err != nil {
		return nil, err
	}
	ov := make([]calendar.Day, 0, len(days))
	for _, d := range days {
		ov = append(ov, calendar.Day{Date: d.Date, Kind: calendar.Kind(d.Kind), Name: d.Name, Source: d.Source})
	}
	return calendar.HolidayFunc(ov), nil
}

func (s *Service) yearFor(ctx context.Context, q store.Queries, emp model.Employee, start time.Time) (model.LeaveYear, error) {
	if emp.HireDate == nil {
		return model.LeaveYear{}, code("hire_date_required", "vacation requires a hire date from the directory", 400)
	}
	years, err := q.ListLeaveYears(ctx, emp.ID)
	if err != nil {
		return model.LeaveYear{}, err
	}
	start = leave.Date(start)
	for _, y := range years {
		if inWorkingYear(start, y.StartsOn, y.EndsOn) {
			return y, nil
		}
	}
	ys, ye := leave.WorkingYearBounds(*emp.HireDate, start)
	now := s.now()
	y := model.LeaveYear{
		ID: s.id(), EmployeeID: emp.ID, StartsOn: ys, EndsOn: ye,
		GrantedDays: leave.BasicDays, ExtraDays: emp.ExtraDays,
		CreatedAt: now, UpdatedAt: now,
	}
	return q.UpsertLeaveYear(ctx, y)
}

func (s *Service) ensureAdjacentYears(ctx context.Context, q store.Queries, emp model.Employee) error {
	if emp.HireDate == nil {
		return nil
	}
	today := s.now()
	_, err := s.yearFor(ctx, q, emp, today)
	if err != nil {
		return err
	}
	next := leave.NextWorkingYearStart(*emp.HireDate, today)
	_, err = s.yearFor(ctx, q, emp, next)
	return err
}

type dayStat struct {
	used     int
	reserved int
	parts    []leave.Part
	unpaid   int
	hadLeave bool
}

func (s *Service) statsForYear(reqs []model.Request, year model.LeaveYear, holiday func(time.Time) bool, sicks []model.Request, releaseID string) dayStat {
	var st dayStat
	for _, r := range reqs {
		if r.ID == releaseID || !inWorkingYear(r.StartsOn, year.StartsOn, year.EndsOn) {
			continue
		}
		if r.Type == model.TypeDayOff && approvedStatus(r.Status) {
			if spec, ok := lookupReason(r.ReasonCode); ok && spec.Unpaid {
				st.unpaid += leave.DaysInclusive(r.StartsOn, r.EndsOn)
			}
		}
		if r.Type != model.TypeVacation || !countsReserved(r) {
			continue
		}
		n := requestDays(r, holiday, sicks)
		if approvedStatus(r.Status) {
			st.used += n
			st.hadLeave = st.hadLeave || n > 0
		} else if inFlight(r.Status) {
			st.reserved += n
		}
		st.parts = append(st.parts, leave.Part{CountedDays: n, Active: true})
	}
	return st
}

func (s *Service) checkVacation(emp model.Employee, year model.LeaveYear, reqs []model.Request, start, end time.Time, newDays int, releaseID string, holiday func(time.Time) bool, sicks []model.Request) error {
	if leave.CrossesYear(start, end, year.StartsOn, year.EndsOn) {
		return code("crosses_working_year", "a request must stay inside one working year", 400)
	}
	if newDays <= 0 {
		return code("empty_range", "the range has no counted vacation days", 400)
	}
	st := s.statsForYear(reqs, year, holiday, sicks, releaseID)
	grant := year.GrantedDays + year.ExtraDays
	if grant == 0 {
		grant = leave.BasicDays + emp.ExtraDays
	}
	excess := leave.UnpaidExcess(st.unpaid)
	first := emp.HireDate != nil && leave.IsFirstWorkingYear(*emp.HireDate, year.StartsOn) && leave.WithinFirstSixMonths(*emp.HireDate, s.now(), excess)
	num, den := leave.Accrued(grant, year.StartsOn, year.EndsOn, s.now(), excess)
	accrued := float64(num) / float64(den)
	if !leave.BalanceAllows(newDays, accrued, grant, st.used+st.reserved, first, emp.EarlyLeave) {
		if first && !emp.EarlyLeave {
			return code("first_six_months", "during the first six months the request cannot exceed accrued days", 400)
		}
		return code("balance", "the request exceeds the working-year balance", 400)
	}
	parts := append(st.parts, leave.Part{CountedDays: newDays, Active: true})
	if !leave.StreakPossible(parts, grant, st.used+st.reserved+newDays) {
		return code("streak_impossible", "this request would leave no 14-day vacation part in the working year", 400)
	}
	return nil
}

func (s *Service) audit(ctx context.Context, q store.Queries, actor, requestID, employeeID, action, before, after string) error {
	detail := []byte(`{}`)
	return q.InsertAudit(ctx, model.Audit{
		ID: s.id(), ActorID: actor, RequestID: requestID, EmployeeID: employeeID,
		Action: action, BeforeStatus: before, AfterStatus: after, Detail: detail, CreatedAt: s.now(),
	})
}

func (s *Service) present(r model.Request, aud string, holiday func(time.Time) bool, sicks []model.Request, year *model.LeaveYear) RequestView {
	days := r.CountedDays
	if r.Status != model.StatusCancelled && r.Status != model.StatusRejected {
		days = requestDays(r, holiday, sicks)
	}
	v := RequestView{
		ID: r.ID, Type: string(r.Type), Status: string(shownStatus(r, s.now())),
		EmployeeID: r.EmployeeID, StartsOn: leave.FormatDate(r.StartsOn), EndsOn: leave.FormatDate(r.EndsOn),
		CountedDays: days, ReasonCode: r.ReasonCode, ReplacesRequestID: r.ReplacesID,
		SuccessorRequestID: r.SuccessorID, CancelReason: r.CancelReason, OverlapChoice: r.OverlapChoice,
		ManagerVacant: r.ManagerVacant, SelfApproval: r.SelfApproval, NeedsConfirmation: r.NeedsConfirmation,
		PayByWarning: r.PayByWarning, ProcessedBy: r.ProcessedBy,
		AcknowledgedAt: stamp(r.AcknowledgedAt), ConfirmedAt: stamp(r.ConfirmedAt),
		ProcessedAt: stamp(r.ProcessedAt), CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339),
	}
	showCert := aud == "self" || aud == "hr" || (aud == "manager" && r.Status == model.StatusPendingManager)
	if showCert {
		v.CertificateNumber = r.CertificateNumber
	}
	if r.Type == model.TypeVacation {
		v.PayByDate = leave.FormatDate(leave.PayByDate(r.StartsOn))
		v.StatutoryPayByDate = leave.FormatDate(leave.StatutoryPayByDate(r.StartsOn))
	}
	if year != nil {
		v.WorkingYearStart = leave.FormatDate(year.StartsOn)
		v.WorkingYearEnd = leave.FormatDate(year.EndsOn)
	}
	return v
}

func (s *Service) refreshConfirmation(ctx context.Context, q store.Queries, r *model.Request) error {
	if r.NeedsConfirmation && r.ConfirmedAt == nil && r.Status == model.StatusScheduled && leave.ConfirmationOpen(s.now(), r.StartsOn) {
		before := r.Status
		r.Status = model.StatusAwaitingConfirmation
		r.UpdatedAt = s.now()
		if err := q.UpdateRequest(ctx, *r); err != nil {
			return err
		}
		return s.audit(ctx, q, "", r.ID, r.EmployeeID, "awaiting_confirmation", string(before), string(r.Status))
	}
	return nil
}

func managerStill(emp, mgr model.Employee) bool {
	return emp.ManagerID != "" && emp.ManagerID != emp.ID && mgr.Active && mgr.ID == emp.ManagerID
}
