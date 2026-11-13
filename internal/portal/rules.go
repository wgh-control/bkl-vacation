package portal

import (
	"time"

	"github.com/wgh-control/bkl-vacation/internal/leave"
	"github.com/wgh-control/bkl-vacation/internal/model"
)

type reasonSpec struct {
	Code      string
	Unpaid    bool
	Mandatory bool
	MaxDays   int
}

var reasonCatalog = []reasonSpec{
	{Code: "unpaid_personal", Unpaid: true},
	{Code: "unpaid_family", Unpaid: true, Mandatory: true, MaxDays: 5},
	{Code: "compensatory_overtime"},
	{Code: "compensatory_weekend"},
}

func lookupReason(code string) (reasonSpec, bool) {
	for _, r := range reasonCatalog {
		if r.Code == code {
			return r, true
		}
	}
	return reasonSpec{}, false
}

func reserves(st model.Status) bool {
	switch st {
	case model.StatusDraft, model.StatusRejected, model.StatusCancelled:
		return false
	default:
		return true
	}
}

func inFlight(st model.Status) bool {
	switch st {
	case model.StatusDraft, model.StatusPendingManager, model.StatusPendingHR:
		return true
	default:
		return false
	}
}

func approvedStatus(st model.Status) bool {
	switch st {
	case model.StatusScheduled, model.StatusAwaitingConfirmation, model.StatusConfirmed, model.StatusProcessed:
		return true
	default:
		return false
	}
}

func countsReserved(r model.Request) bool {
	if r.Type != model.TypeVacation || !reserves(r.Status) {
		return false
	}
	// A move that is still waiting does not reserve a second time.
	// The original request keeps the days until the move is approved.
	if r.ReplacesID != "" && !approvedStatus(r.Status) {
		return false
	}
	return true
}

func vacationDates(r model.Request, holiday func(time.Time) bool, sicks []model.Request) []time.Time {
	if r.Type != model.TypeVacation {
		return leave.CalendarDays(r.StartsOn, r.EndsOn)
	}
	var out []time.Time
	leave.EachDay(r.StartsOn, r.EndsOn, func(d time.Time) {
		if holiday != nil && holiday(d) {
			return
		}
		for _, s := range sicks {
			if s.ID == r.ID || s.Type != model.TypeSick || !approvedStatus(s.Status) {
				continue
			}
			if !d.Before(leave.Date(s.StartsOn)) && !d.After(leave.Date(s.EndsOn)) {
				return
			}
		}
		out = append(out, d)
	})
	return out
}

func requestDays(r model.Request, holiday func(time.Time) bool, sicks []model.Request) int {
	return len(vacationDates(r, holiday, sicks))
}

func inWorkingYear(start, yearStart, yearEnd time.Time) bool {
	start = leave.Date(start)
	return !start.Before(leave.Date(yearStart)) && !start.After(leave.Date(yearEnd))
}

func rangesConflict(existing model.Request, start, end time.Time, typ model.RequestType, ignoreID string) bool {
	if existing.ID == ignoreID || existing.Status == model.StatusRejected || existing.Status == model.StatusCancelled {
		return false
	}
	if !leave.Overlaps(existing.StartsOn, existing.EndsOn, start, end) {
		return false
	}
	if typ == model.TypeSick && existing.Type == model.TypeVacation && approvedStatus(existing.Status) {
		return false
	}
	return true
}

func shownStatus(r model.Request, now time.Time) model.Status {
	if r.NeedsConfirmation && r.ConfirmedAt == nil &&
		(r.Status == model.StatusScheduled || r.Status == model.StatusAwaitingConfirmation) &&
		leave.ConfirmationOpen(now, r.StartsOn) {
		return model.StatusAwaitingConfirmation
	}
	return r.Status
}
