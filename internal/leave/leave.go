// Package leave is the Labor Code arithmetic for the portal.
// It has no HTTP and no database. Articles are the Labor Code of the
// Russian Federation unless a comment names another act.
package leave

import (
	"math"
	"time"
)

const (
	// BasicDays is the annual basic paid leave (article 115).
	BasicDays = 28
	// StreakDays is the one part that must be at least this long (article 125).
	StreakDays = 14
	// UnpaidServiceCap is the unpaid leave that still counts as qualifying
	// service in a working year (article 121).
	UnpaidServiceCap = 14
	// CarryoverWarnDays is how close the article 124 deadline must be before HR is warned.
	CarryoverWarnDays = 60
	// FirstYearMonths is the continuous service after which the full grant may be taken (article 122).
	FirstYearMonths = 6
)

// Date returns the civil day in UTC.
func Date(t time.Time) time.Time {
	y, m, d := t.In(time.UTC).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// ParseDate parses YYYY-MM-DD.
func ParseDate(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, err
	}
	return Date(t), nil
}

// FormatDate formats a civil day.
func FormatDate(t time.Time) string {
	return Date(t).Format("2006-01-02")
}

// EachDay walks the inclusive range.
func EachDay(start, end time.Time, fn func(time.Time)) {
	start, end = Date(start), Date(end)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		fn(d)
	}
}

// DaysInclusive is the number of calendar days in the inclusive range.
func DaysInclusive(start, end time.Time) int {
	start, end = Date(start), Date(end)
	if end.Before(start) {
		return 0
	}
	return int(end.Sub(start).Hours()/24) + 1
}

// Overlaps reports whether two inclusive ranges share a day.
func Overlaps(aStart, aEnd, bStart, bEnd time.Time) bool {
	aStart, aEnd = Date(aStart), Date(aEnd)
	bStart, bEnd = Date(bStart), Date(bEnd)
	return !aStart.After(bEnd) && !bStart.After(aEnd)
}

// WorkingYearBounds is the working year that contains on, counted from hire
// (articles 121 and 122). The end is inclusive.
func WorkingYearBounds(hire, on time.Time) (start, end time.Time) {
	hire, on = Date(hire), Date(on)
	if on.Before(hire) {
		on = hire
	}
	start = time.Date(on.Year(), hire.Month(), hire.Day(), 0, 0, 0, 0, time.UTC)
	if start.After(on) {
		start = time.Date(on.Year()-1, hire.Month(), hire.Day(), 0, 0, 0, 0, time.UTC)
	}
	end = start.AddDate(1, 0, -1)
	return start, end
}

// NextWorkingYearStart is the day after the inclusive end of the year that contains on.
func NextWorkingYearStart(hire, on time.Time) time.Time {
	_, end := WorkingYearBounds(hire, on)
	return end.AddDate(0, 0, 1)
}

// PreviousWorkingYearStart is the start of the working year before the one that contains on.
// ok is false when that year would begin before the hire date.
func PreviousWorkingYearStart(hire, on time.Time) (time.Time, bool) {
	start, _ := WorkingYearBounds(hire, on)
	prev := start.AddDate(0, 0, -1)
	if prev.Before(Date(hire)) {
		return time.Time{}, false
	}
	ps, _ := WorkingYearBounds(hire, prev)
	return ps, true
}

// UnpaidExcess is the unpaid calendar days that do not count as qualifying service.
func UnpaidExcess(unpaidCalendarDays int) int {
	if unpaidCalendarDays <= UnpaidServiceCap {
		return 0
	}
	return unpaidCalendarDays - UnpaidServiceCap
}

// AccrualAdjustment is the reduction of accrued vacation for unpaid excess
// (article 121). It is negative or zero. The annual grant itself stays 28
// plus any HR extra; this figure is the part of the grant not yet earned
// because those days are outside qualifying service.
func AccrualAdjustment(granted, yearLength, excess int) float64 {
	if granted <= 0 || yearLength <= 0 || excess <= 0 {
		return 0
	}
	if excess > yearLength {
		excess = yearLength
	}
	return -float64(granted) * float64(excess) / float64(yearLength)
}

// Accrued returns earned days as a rational granted*qualifying/length.
// qualifying is calendar days from the working-year start through asOf, minus excess.
func Accrued(granted int, yearStart, yearEnd, asOf time.Time, excess int) (num, den int) {
	yearStart, yearEnd, asOf = Date(yearStart), Date(yearEnd), Date(asOf)
	den = DaysInclusive(yearStart, yearEnd)
	if den <= 0 || granted <= 0 {
		return 0, 1
	}
	if asOf.Before(yearStart) {
		return 0, den
	}
	until := asOf
	if until.After(yearEnd) {
		until = yearEnd
	}
	qualifying := DaysInclusive(yearStart, until) - excess
	if qualifying < 0 {
		qualifying = 0
	}
	if qualifying > den {
		qualifying = den
	}
	return granted * qualifying, den
}

// Decimal formats num/den half-up to places decimal digits.
func Decimal(num, den, places int) float64 {
	if den == 0 {
		return 0
	}
	scale := math.Pow10(places)
	v := float64(num) / float64(den) * scale
	if v >= 0 {
		return math.Round(v) / scale
	}
	return -math.Round(-v) / scale
}

// WithinFirstSixMonths reports whether qualifying service is still under six months
// (article 122). Excess unpaid days push the clock back.
func WithinFirstSixMonths(hire, asOf time.Time, excess int) bool {
	hire = Date(hire)
	qualified := Date(asOf).AddDate(0, 0, -excess)
	return qualified.Before(hire.AddDate(0, FirstYearMonths, 0))
}

// IsFirstWorkingYear reports whether yearStart is the hire date's first working year.
func IsFirstWorkingYear(hire, yearStart time.Time) bool {
	return Date(yearStart).Equal(Date(hire))
}

// PayByDate is the stricter article 136 date. Rostrud treats the three days as
// calendar days. A Mintrud statement reported on 8 November 2026 says the
// payment day itself is not counted inside those three days, so the last day
// the company can pay is four calendar days before the start (D8).
func PayByDate(start time.Time) time.Time {
	return Date(start).AddDate(0, 0, -4)
}

// StatutoryPayByDate is start minus three calendar days (Rostrud 30 July 2014 No. 1693-6-1).
func StatutoryPayByDate(start time.Time) time.Time {
	return Date(start).AddDate(0, 0, -3)
}

// PayByMissed reports that a request filed on today cannot meet the stricter pay-by date.
func PayByMissed(today, start time.Time) bool {
	today, start = Date(today), Date(start)
	if !start.After(today) {
		return true
	}
	return today.After(PayByDate(start))
}

// NoticeDue is fourteen calendar days before the start (article 123).
func NoticeDue(start time.Time) time.Time {
	return Date(start).AddDate(0, 0, -14)
}

// ConfirmOpen is five calendar days before the start, the opening of the company window.
func ConfirmOpen(start time.Time) time.Time {
	return Date(start).AddDate(0, 0, -5)
}

// RemindDue is three calendar days before the start.
func RemindDue(start time.Time) time.Time {
	return Date(start).AddDate(0, 0, -3)
}

// ConfirmationOpen reports whether today is on or after the five-day mark.
// The window stays open after the start so a late confirmation can still be recorded (D7).
func ConfirmationOpen(today, start time.Time) bool {
	return !Date(today).Before(ConfirmOpen(start))
}

// DaysUntil is the number of calendar days from today to start. Zero means today.
func DaysUntil(today, start time.Time) int {
	today, start = Date(today), Date(start)
	return int(start.Sub(today).Hours() / 24)
}

// PlanningHorizon allows a new vacation or day off in the current calendar year
// or the next one, and not in the past. Sick leave may start up to one year in the past.
func PlanningHorizon(today, start, end time.Time, allowPast bool) bool {
	today, start, end = Date(today), Date(start), Date(end)
	if end.Before(start) {
		return false
	}
	if start.Year() > today.Year()+1 || end.Year() > today.Year()+1 {
		return false
	}
	if !allowPast && start.Before(today) {
		return false
	}
	if allowPast && start.Before(today.AddDate(-1, 0, 0)) {
		return false
	}
	return true
}

// Part is one vacation request considered by the 14-day rule.
type Part struct {
	CountedDays int
	Active      bool
}

// StreakPossible reports whether the working year already has a part of at least
// 14 counted days, or still has enough free days to form one (article 125).
func StreakPossible(parts []Part, granted, reservedIncludingNew int) bool {
	for _, p := range parts {
		if p.Active && p.CountedDays >= StreakDays {
			return true
		}
	}
	return granted-reservedIncludingNew >= StreakDays
}

// CarryoverDeadline is twelve months after the inclusive end of the working year (article 124).
func CarryoverDeadline(yearEnd time.Time) time.Time {
	return Date(yearEnd).AddDate(1, 0, 0)
}

// CarryoverWarn reports whether unused days are inside the 60-day warning window
// or already past the article 124 deadline.
func CarryoverWarn(today, yearEnd time.Time, unused int) bool {
	if unused <= 0 {
		return false
	}
	deadline := CarryoverDeadline(yearEnd)
	today = Date(today)
	return !deadline.After(today.AddDate(0, 0, CarryoverWarnDays))
}

// TwoYearWarning is the article 124 ban on going two consecutive working years without annual leave.
// The portal warns; it does not block.
func TwoYearWarning(previousHadLeave, currentHadLeave bool) bool {
	return !previousHadLeave && !currentHadLeave
}

// CountedVacationDays lists calendar days in the range except non-working holidays
// (article 120). Weekends stay in the list.
func CountedVacationDays(start, end time.Time, holiday func(time.Time) bool) []time.Time {
	var out []time.Time
	EachDay(start, end, func(d time.Time) {
		if holiday != nil && holiday(d) {
			return
		}
		out = append(out, d)
	})
	return out
}

// EndForCountedDays walks forward from start until want counted days are collected.
// Holidays inside the span are skipped and the end moves (article 120).
func EndForCountedDays(start time.Time, want int, holiday func(time.Time) bool) (time.Time, bool) {
	if want <= 0 {
		return time.Time{}, false
	}
	got := 0
	d := Date(start)
	for i := 0; i < 400 && got < want; i++ {
		if holiday == nil || !holiday(d) {
			got++
			if got == want {
				return d, true
			}
		}
		d = d.AddDate(0, 0, 1)
	}
	return time.Time{}, false
}

// ExtendEnd moves the inclusive end forward by extraCounted days, skipping holidays.
func ExtendEnd(end time.Time, extraCounted int, holiday func(time.Time) bool) time.Time {
	d := Date(end)
	added := 0
	for added < extraCounted {
		d = d.AddDate(0, 0, 1)
		if holiday == nil || !holiday(d) {
			added++
		}
	}
	return d
}

// SplitByYear counts dates per calendar year.
func SplitByYear(dates []time.Time) map[int]int {
	m := map[int]int{}
	for _, d := range dates {
		m[d.Year()]++
	}
	return m
}

// CalendarDays lists every civil day in the inclusive range.
func CalendarDays(start, end time.Time) []time.Time {
	var out []time.Time
	EachDay(start, end, func(d time.Time) { out = append(out, d) })
	return out
}

// IntersectDates returns days present in both lists.
func IntersectDates(a, b []time.Time) []time.Time {
	set := map[string]struct{}{}
	for _, d := range a {
		set[FormatDate(d)] = struct{}{}
	}
	var out []time.Time
	seen := map[string]struct{}{}
	for _, d := range b {
		k := FormatDate(d)
		if _, ok := set[k]; ok {
			if _, dup := seen[k]; dup {
				continue
			}
			seen[k] = struct{}{}
			out = append(out, Date(d))
		}
	}
	return out
}

// BalanceAllows reports whether requested days fit.
// Inside the first six months, without an early-leave exception, the person
// cannot take more than the accrued free balance. After that, the full grant
// may be taken in advance (article 122).
func BalanceAllows(requested int, accrued float64, grant, alreadyReserved int, firstSix, early bool) bool {
	if requested < 0 {
		return false
	}
	used := alreadyReserved + requested
	if firstSix && !early {
		return float64(used) <= accrued+1e-9
	}
	return used <= grant
}

// CrossesYear reports whether an inclusive range is not fully inside the working year.
func CrossesYear(start, end, yearStart, yearEnd time.Time) bool {
	start, end = Date(start), Date(end)
	yearStart, yearEnd = Date(yearStart), Date(yearEnd)
	return start.Before(yearStart) || end.After(yearEnd)
}
