package portal

import (
	"context"
	"time"

	"github.com/wgh-control/bkl-vacation/internal/identity"
	"github.com/wgh-control/bkl-vacation/internal/leave"
	"github.com/wgh-control/bkl-vacation/internal/model"
	"github.com/wgh-control/bkl-vacation/internal/store"
)

// Balance is the working-year balance plus the calendar-year day buckets.
func (s *Service) Balance(ctx context.Context, p identity.Principal) (Balance, error) {
	var out Balance
	err := s.tx(ctx, func(q store.Queries) error {
		emp, err := s.require(ctx, q, p, false)
		if err != nil {
			return err
		}
		out, err = s.balanceFor(ctx, q, emp)
		return err
	})
	return out, err
}

// YearDays returns planned, approved, and moved vacation days for past years,
// the current year, and next year.
func (s *Service) YearDays(ctx context.Context, p identity.Principal) (YearSummary, error) {
	bal, err := s.Balance(ctx, p)
	if err != nil {
		return YearSummary{}, err
	}
	return YearSummary{AsOf: leave.FormatDate(s.now()), Years: bal.CalendarYears}, nil
}

func (s *Service) balanceFor(ctx context.Context, q store.Queries, emp model.Employee) (Balance, error) {
	if emp.HireDate != nil {
		if _, err := s.yearFor(ctx, q, emp, s.now()); err != nil {
			return Balance{}, err
		}
		if _, err := s.yearFor(ctx, q, emp, leave.NextWorkingYearStart(*emp.HireDate, s.now())); err != nil {
			return Balance{}, err
		}
		if prev, ok := leave.PreviousWorkingYearStart(*emp.HireDate, s.now()); ok {
			if _, err := s.yearFor(ctx, q, emp, prev); err != nil {
				return Balance{}, err
			}
		}
	}
	years, err := q.ListLeaveYears(ctx, emp.ID)
	if err != nil {
		return Balance{}, err
	}
	reqs, err := q.ListRequests(ctx, model.RequestFilter{EmployeeID: emp.ID})
	if err != nil {
		return Balance{}, err
	}
	from := s.now().AddDate(-2, 0, 0)
	to := s.now().AddDate(2, 0, 0)
	if len(years) > 0 {
		from = years[0].StartsOn.AddDate(0, 0, -5)
		to = years[len(years)-1].EndsOn.AddDate(0, 0, 40)
	}
	hol, err := s.holidayFn(ctx, q, from, to)
	if err != nil {
		return Balance{}, err
	}
	sicks := sickRequests(reqs)
	working := make([]WorkingYearView, 0, len(years))
	var prevHad *bool
	now := s.now()
	for _, y := range years {
		st := s.statsForYear(reqs, y, hol, sicks, "")
		grant := y.GrantedDays + y.ExtraDays
		if y.GrantedDays == 0 {
			grant = leave.BasicDays + y.ExtraDays
		}
		excess := leave.UnpaidExcess(st.unpaid)
		length := leave.DaysInclusive(y.StartsOn, y.EndsOn)
		num, den := leave.Accrued(grant, y.StartsOn, y.EndsOn, now, excess)
		adj := leave.AccrualAdjustment(grant, length, excess)
		accrued := leave.Decimal(num, den, 4)
		free := grant - st.used - st.reserved
		advance := float64(st.used+st.reserved) - accrued
		if advance < 0 {
			advance = 0
		}
		satisfied := false
		for _, part := range st.parts {
			if part.Active && part.CountedDays >= leave.StreakDays {
				satisfied = true
			}
		}
		unused := free
		if unused < 0 {
			unused = 0
		}
		two := false
		if prevHad != nil && !*prevHad && !st.hadLeave && !y.StartsOn.After(leave.Date(now)) {
			two = true
		}
		had := st.hadLeave
		prevHad = &had
		y.UnpaidCalendarDays = st.unpaid
		y.ExcessUnpaidDays = excess
		y.AccruedAdjustment = adj
		y.UpdatedAt = now
		if _, err := q.UpsertLeaveYear(ctx, y); err != nil {
			return Balance{}, err
		}
		base := y.GrantedDays
		if base == 0 {
			base = leave.BasicDays
		}
		working = append(working, WorkingYearView{
			StartsOn: leave.FormatDate(y.StartsOn), EndsOn: leave.FormatDate(y.EndsOn),
			GrantedDays: base, ExtraDays: y.ExtraDays,
			AccruedDays: accrued, AccruedNumerator: num, AccruedDenominator: den, AccruedAdjustment: adj,
			UsedDays: st.used, ReservedDays: st.reserved, FreeDays: free, AdvanceDays: advance,
			StreakSatisfied: satisfied, StreakPossible: leave.StreakPossible(st.parts, grant, st.used+st.reserved),
			CarryoverWarning:  leave.CarryoverWarn(now, y.EndsOn, unused),
			CarryoverDeadline: leave.FormatDate(leave.CarryoverDeadline(y.EndsOn)),
			TwoYearWarning:    two, UnpaidCalendarDays: st.unpaid, UnpaidExcessDays: excess,
		})
	}
	cal := calendarBuckets(s.now().Year(), reqs, hol)
	if working == nil {
		working = []WorkingYearView{}
	}
	return Balance{WorkingYears: working, CalendarYears: cal}, nil
}

func calendarBuckets(current int, reqs []model.Request, hol func(time.Time) bool) []YearDays {
	earliest := current - 1
	for _, r := range reqs {
		if y := r.StartsOn.Year(); y < earliest {
			earliest = y
		}
	}
	sicks := sickRequests(reqs)
	var out []YearDays
	for year := earliest; year <= current+1; year++ {
		item := YearDays{Year: year, Relation: relation(year, current), Entries: []YearEntry{}}
		for _, r := range reqs {
			if r.Status == model.StatusRejected || (r.Status == model.StatusCancelled && r.CancelReason != model.CancelRescheduled) {
				continue
			}
			dates := vacationDates(r, hol, sicks)
			if r.Type != model.TypeVacation {
				dates = leave.CalendarDays(r.StartsOn, r.EndsOn)
			}
			inYear := 0
			for _, d := range dates {
				if d.Year() == year {
					inYear++
				}
			}
			if inYear == 0 {
				continue
			}
			entry := YearEntry{
				RequestID: r.ID, Type: string(r.Type), Status: string(r.Status),
				StartsOn: leave.FormatDate(r.StartsOn), EndsOn: leave.FormatDate(r.EndsOn),
				Days: inYear, Reschedule: r.ReplacesID != "" || r.CancelReason == model.CancelRescheduled,
			}
			switch {
			case r.Status == model.StatusCancelled && r.CancelReason == model.CancelRescheduled:
				entry.Bucket = "moved_away"
				if r.Type == model.TypeVacation {
					item.MovedDays += inYear
					item.MovedAwayDays += inYear
				}
			case inFlight(r.Status):
				entry.Bucket = "planned"
				if r.Type == model.TypeVacation {
					item.PlannedDays += inYear
					if r.ReplacesID != "" {
						item.MovedDays += inYear
					}
				}
			case approvedStatus(r.Status):
				entry.Bucket = "approved"
				if r.Type == model.TypeVacation {
					item.ApprovedDays += inYear
					if r.ReplacesID != "" {
						item.MovedDays += inYear
					}
				}
			}
			if r.Type == model.TypeDayOff && r.Status != model.StatusCancelled {
				item.DayOffDays += inYear
			}
			if r.Type == model.TypeSick && r.Status != model.StatusCancelled {
				item.SickDays += inYear
			}
			item.Entries = append(item.Entries, entry)
		}
		out = append(out, item)
	}
	return out
}

func relation(year, current int) string {
	switch {
	case year < current:
		return "past"
	case year > current:
		return "next"
	default:
		return "current"
	}
}
