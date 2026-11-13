package leave

import (
	"testing"
	"time"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func holidays(days ...time.Time) func(time.Time) bool {
	set := map[string]struct{}{}
	for _, day := range days {
		set[FormatDate(day)] = struct{}{}
	}
	return func(t time.Time) bool {
		_, ok := set[FormatDate(t)]
		return ok
	}
}

func TestWorkingYearFromHireDate(t *testing.T) {
	hire := d(2020, 3, 15)
	start, end := WorkingYearBounds(hire, d(2026, 11, 13))
	if !start.Equal(d(2026, 3, 15)) || !end.Equal(d(2027, 3, 14)) {
		t.Fatalf("current year %s..%s", FormatDate(start), FormatDate(end))
	}
	start, end = WorkingYearBounds(hire, d(2026, 2, 1))
	if !start.Equal(d(2025, 3, 15)) || !end.Equal(d(2026, 3, 14)) {
		t.Fatalf("previous side %s..%s", FormatDate(start), FormatDate(end))
	}
	start, end = WorkingYearBounds(hire, d(2020, 3, 15))
	if !start.Equal(hire) || !IsFirstWorkingYear(hire, start) {
		t.Fatalf("first year %s", FormatDate(start))
	}
}

func TestWeekendsCountHolidaysDoNot(t *testing.T) {
	// Friday 13 Nov 2026 through Monday 16 Nov 2026. 14 and 15 are the weekend.
	got := CountedVacationDays(d(2026, 11, 13), d(2026, 11, 16), holidays())
	if len(got) != 4 {
		t.Fatalf("weekend days count, got %d", len(got))
	}
	got = CountedVacationDays(d(2026, 12, 30), d(2027, 1, 10), holidays(
		d(2027, 1, 1), d(2027, 1, 2), d(2027, 1, 3), d(2027, 1, 4),
		d(2027, 1, 5), d(2027, 1, 6), d(2027, 1, 7), d(2027, 1, 8),
	))
	if len(got) != 4 {
		t.Fatalf("holidays excluded, got %d (%v)", len(got), got)
	}
}

func TestEndMovesPastHolidays(t *testing.T) {
	h := holidays(d(2027, 1, 1), d(2027, 1, 2), d(2027, 1, 7))
	end, ok := EndForCountedDays(d(2027, 1, 1), 2, h)
	if !ok || !end.Equal(d(2027, 1, 4)) {
		t.Fatalf("end %s ok %v", FormatDate(end), ok)
	}
	ext := ExtendEnd(d(2027, 1, 6), 2, h)
	if !ext.Equal(d(2027, 1, 9)) {
		t.Fatalf("extension %s", FormatDate(ext))
	}
}

func TestStreakRule(t *testing.T) {
	grant := BasicDays
	// A 10-day part leaves 18 free, so a 14-day part is still possible.
	if !StreakPossible(nil, grant, 10) {
		t.Fatal("10 days must leave a streak possible")
	}
	// Two 10-day parts would be reserved 20, free 8, and neither part is 14.
	parts := []Part{{CountedDays: 10, Active: true}}
	if StreakPossible(parts, grant, 20) {
		t.Fatal("second 10-day part must be rejected")
	}
	// An existing 14-day part allows a one-day part.
	parts = []Part{{CountedDays: 14, Active: true}, {CountedDays: 1, Active: true}}
	if !StreakPossible(parts, grant, 15) {
		t.Fatal("streak already taken")
	}
	// The new request itself can be the streak.
	if !StreakPossible([]Part{{CountedDays: 14, Active: true}}, grant, 14) {
		t.Fatal("the 14-day request is the streak")
	}
}

func TestArticle121UnpaidExcess(t *testing.T) {
	if UnpaidExcess(14) != 0 {
		t.Fatal("14 unpaid days still qualify")
	}
	if UnpaidExcess(20) != 6 {
		t.Fatalf("excess %d", UnpaidExcess(20))
	}
	adj := AccrualAdjustment(28, 365, 6)
	if adj >= 0 {
		t.Fatal("adjustment reduces accrual")
	}
	want := -28.0 * 6 / 365
	if mathAbs(adj-want) > 1e-9 {
		t.Fatalf("adjustment %v want %v", adj, want)
	}
	num, den := Accrued(28, d(2026, 1, 1), d(2026, 12, 31), d(2026, 12, 31), 6)
	if num != 28*(365-6) || den != 365 {
		t.Fatalf("accrued %d/%d", num, den)
	}
	// Compensatory time off is not unpaid leave; the caller passes only unpaid days.
}

func TestFirstSixMonths(t *testing.T) {
	hire := d(2026, 8, 1)
	asOf := d(2026, 11, 13)
	if !WithinFirstSixMonths(hire, asOf, 0) {
		t.Fatal("November is still inside six months from August")
	}
	if WithinFirstSixMonths(hire, d(2027, 2, 1), 0) {
		t.Fatal("February is past six months")
	}
	// Excess unpaid days keep the person inside the first six months.
	if !WithinFirstSixMonths(hire, d(2027, 2, 1), 40) {
		t.Fatal("excess should push the clock back")
	}
	num, den := Accrued(28, d(2026, 8, 1), d(2027, 7, 31), asOf, 0)
	accrued := float64(num) / float64(den)
	if BalanceAllows(10, accrued, 28, 0, true, false) {
		t.Fatal("advance during the first six months is rejected")
	}
	if !BalanceAllows(1, accrued, 28, 0, true, false) {
		t.Fatal("a day within accrual is allowed")
	}
	if !BalanceAllows(28, accrued, 28, 0, true, true) {
		t.Fatal("early leave may take the full grant")
	}
	if !BalanceAllows(28, accrued, 28, 0, false, false) {
		t.Fatal("after six months the full 28 may be taken in advance")
	}
	if BalanceAllows(29, 28, 28, 0, false, false) {
		t.Fatal("the grant is still the cap")
	}
}

func TestPayByAndCarryover(t *testing.T) {
	start := d(2026, 11, 20)
	if !PayByDate(start).Equal(d(2026, 11, 16)) {
		t.Fatalf("stricter pay-by %s", FormatDate(PayByDate(start)))
	}
	if !StatutoryPayByDate(start).Equal(d(2026, 11, 17)) {
		t.Fatalf("statutory pay-by %s", FormatDate(StatutoryPayByDate(start)))
	}
	if !PayByMissed(d(2026, 11, 18), start) {
		t.Fatal("filing after the stricter date warns")
	}
	if PayByMissed(d(2026, 11, 16), start) {
		t.Fatal("filing on the pay-by date still meets it")
	}
	yearEnd := d(2026, 3, 14)
	deadline := CarryoverDeadline(yearEnd)
	if !deadline.Equal(d(2027, 3, 14)) {
		t.Fatalf("deadline %s", FormatDate(deadline))
	}
	if !CarryoverWarn(d(2027, 2, 1), yearEnd, 5) {
		t.Fatal("inside 60 days")
	}
	if !CarryoverWarn(d(2027, 4, 1), yearEnd, 5) {
		t.Fatal("overdue unused days stay visible")
	}
	if CarryoverWarn(d(2026, 11, 13), yearEnd, 5) {
		t.Fatal("far from the deadline")
	}
	if CarryoverWarn(d(2027, 3, 1), yearEnd, 0) {
		t.Fatal("nothing unused")
	}
	if !TwoYearWarning(false, false) {
		t.Fatal("two empty years warn")
	}
	if TwoYearWarning(true, false) {
		t.Fatal("a previous year with leave is enough")
	}
}

func TestPlanningHorizon(t *testing.T) {
	today := d(2026, 11, 13)
	if !PlanningHorizon(today, d(2027, 6, 1), d(2027, 6, 14), false) {
		t.Fatal("next calendar year is in the planning horizon")
	}
	if !PlanningHorizon(today, d(2026, 12, 20), d(2027, 1, 10), false) {
		t.Fatal("a range that crosses into next year is allowed")
	}
	if PlanningHorizon(today, d(2028, 1, 10), d(2028, 1, 20), false) {
		t.Fatal("the year after next is outside the horizon")
	}
	if PlanningHorizon(today, d(2026, 11, 1), d(2026, 11, 10), false) {
		t.Fatal("vacation cannot start in the past")
	}
	if !PlanningHorizon(today, d(2026, 11, 13), d(2026, 11, 13), false) {
		t.Fatal("today is allowed")
	}
	if !PlanningHorizon(today, d(2026, 5, 1), d(2026, 5, 5), true) {
		t.Fatal("sick leave may start in the past")
	}
	if PlanningHorizon(today, d(2025, 1, 1), d(2025, 1, 2), true) {
		t.Fatal("sick leave is not accepted more than a year back")
	}
}

func TestSickOverlapDays(t *testing.T) {
	vacation := CountedVacationDays(d(2027, 6, 1), d(2027, 6, 14), nil)
	sick := CalendarDays(d(2027, 6, 10), d(2027, 6, 12))
	hit := IntersectDates(vacation, sick)
	if len(hit) != 3 {
		t.Fatalf("overlap %d", len(hit))
	}
	// Postponing returns those days. Extending skips a holiday in the tail.
	end := ExtendEnd(d(2027, 6, 14), len(hit), holidays(d(2027, 6, 16)))
	if !end.Equal(d(2027, 6, 18)) {
		t.Fatalf("extended end %s", FormatDate(end))
	}
}

func TestMinimumPartAndEmptyRange(t *testing.T) {
	if len(CountedVacationDays(d(2027, 1, 1), d(2027, 1, 8), holidays(
		d(2027, 1, 1), d(2027, 1, 2), d(2027, 1, 3), d(2027, 1, 4),
		d(2027, 1, 5), d(2027, 1, 6), d(2027, 1, 7), d(2027, 1, 8),
	))) != 0 {
		t.Fatal("a range that is only holidays is empty")
	}
	one := CountedVacationDays(d(2027, 3, 2), d(2027, 3, 2), nil)
	if len(one) != 1 {
		t.Fatal("the minimum part is one counted day")
	}
}

func TestSplitByYear(t *testing.T) {
	days := CountedVacationDays(d(2026, 12, 30), d(2027, 1, 3), holidays(d(2027, 1, 1)))
	split := SplitByYear(days)
	if split[2026] != 2 || split[2027] != 2 {
		t.Fatalf("split %#v", split)
	}
}

func mathAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
