// Package calendar builds the production calendar: weekends, article 112
// holidays, the default weekend transfer, and HR or government overrides.
package calendar

import (
	"time"

	"github.com/wgh-control/bkl-vacation/internal/leave"
)

type Kind string

const (
	KindWork        Kind = "work"
	KindWeekend     Kind = "weekend"
	KindHoliday     Kind = "holiday"
	KindTransferred Kind = "transferred"
)

type Day struct {
	Date   time.Time
	Kind   Kind
	Name   string
	Source string
}

// FederalHoliday reports an article 112 non-working holiday.
// The list is the federal list as amended by Federal Law of 23 April 2012 No. 35-FZ.
// Government transfers of January weekends are not hard-coded; they are stored as data.
func FederalHoliday(d time.Time) (bool, string) {
	d = leave.Date(d)
	switch {
	case d.Month() == time.January && d.Day() >= 1 && d.Day() <= 6:
		return true, "new_year"
	case d.Month() == time.January && d.Day() == 8:
		return true, "new_year"
	case d.Month() == time.January && d.Day() == 7:
		return true, "christmas"
	case d.Month() == time.February && d.Day() == 23:
		return true, "defender"
	case d.Month() == time.March && d.Day() == 8:
		return true, "womens_day"
	case d.Month() == time.May && d.Day() == 1:
		return true, "spring_labour"
	case d.Month() == time.May && d.Day() == 9:
		return true, "victory"
	case d.Month() == time.June && d.Day() == 12:
		return true, "russia_day"
	case d.Month() == time.November && d.Day() == 4:
		return true, "unity"
	default:
		return false, ""
	}
}

func januaryWindow(d time.Time) bool {
	d = leave.Date(d)
	return d.Month() == time.January && d.Day() >= 1 && d.Day() <= 8
}

func weekend(d time.Time) bool {
	wd := d.Weekday()
	return wd == time.Saturday || wd == time.Sunday
}

// Name returns a display name for a statutory holiday code.
func Name(code, locale string) string {
	ru := locale == "ru" || locale == ""
	switch code {
	case "new_year":
		if ru {
			return "Новогодние каникулы"
		}
		return "New Year holidays"
	case "christmas":
		if ru {
			return "Рождество Христово"
		}
		return "Christmas"
	case "defender":
		if ru {
			return "День защитника Отечества"
		}
		return "Defender of the Fatherland Day"
	case "womens_day":
		if ru {
			return "Международный женский день"
		}
		return "International Women's Day"
	case "spring_labour":
		if ru {
			return "Праздник Весны и Труда"
		}
		return "Spring and Labour Day"
	case "victory":
		if ru {
			return "День Победы"
		}
		return "Victory Day"
	case "russia_day":
		if ru {
			return "День России"
		}
		return "Russia Day"
	case "unity":
		if ru {
			return "День народного единства"
		}
		return "National Unity Day"
	case "weekend":
		if ru {
			return "Выходной"
		}
		return "Weekend"
	case "transferred":
		if ru {
			return "Перенесённый выходной"
		}
		return "Transferred day off"
	default:
		if ru {
			return "Рабочий день"
		}
		return "Working day"
	}
}

// Build returns one row per civil day from start through end.
// Overrides replace the generated kind. A stored holiday is excluded from
// vacation day counts; a transferred rest day is not a holiday and still counts.
func Build(start, end time.Time, overrides []Day, locale string) []Day {
	start, end = leave.Date(start), leave.Date(end)
	if end.Before(start) {
		return nil
	}
	byDate := map[string]Day{}
	var order []time.Time
	leave.EachDay(start, end, func(d time.Time) {
		day := Day{Date: d, Kind: KindWork, Name: Name("work", locale), Source: "calendar"}
		if weekend(d) {
			day.Kind = KindWeekend
			day.Name = Name("weekend", locale)
			day.Source = "weekend"
		}
		if ok, code := FederalHoliday(d); ok {
			day.Kind = KindHoliday
			day.Name = Name(code, locale)
			day.Source = "article_112"
		}
		byDate[leave.FormatDate(d)] = day
		order = append(order, d)
	})
	// Article 112: a weekend that coincides with a holiday moves to the next
	// working day, except weekends inside the 1–8 January holidays. Those
	// January transfers are whatever the government stored for the year.
	for _, d := range order {
		key := leave.FormatDate(d)
		day := byDate[key]
		if day.Kind != KindHoliday || !weekend(d) || januaryWindow(d) {
			continue
		}
		target := nextWorking(d, func(t time.Time) bool {
			if ok, _ := FederalHoliday(t); ok {
				return true
			}
			if got, ok := byDate[leave.FormatDate(t)]; ok && got.Kind == KindHoliday {
				return true
			}
			return false
		})
		if target.IsZero() || target.Before(start) || target.After(end) {
			continue
		}
		tk := leave.FormatDate(target)
		cur := byDate[tk]
		if cur.Kind == KindHoliday || cur.Kind == KindTransferred {
			continue
		}
		cur.Kind = KindTransferred
		cur.Name = Name("transferred", locale)
		cur.Source = "article_112"
		byDate[tk] = cur
	}
	for _, ov := range overrides {
		k := leave.FormatDate(ov.Date)
		cur, ok := byDate[k]
		if !ok {
			continue
		}
		cur.Kind = ov.Kind
		if ov.Name != "" {
			cur.Name = ov.Name
		}
		if ov.Source != "" {
			cur.Source = ov.Source
		}
		byDate[k] = cur
	}
	out := make([]Day, 0, len(order))
	for _, d := range order {
		out = append(out, byDate[leave.FormatDate(d)])
	}
	return out
}

func nextWorking(d time.Time, holiday func(time.Time) bool) time.Time {
	n := leave.Date(d).AddDate(0, 0, 1)
	for i := 0; i < 21; i++ {
		if !weekend(n) && !holiday(n) {
			return n
		}
		n = n.AddDate(0, 0, 1)
	}
	return time.Time{}
}

// HolidayFunc is true on days that do not count as vacation (article 120).
// Stored overrides win over the statutory list. Transferred rest days count.
func HolidayFunc(overrides []Day) func(time.Time) bool {
	ov := map[string]Kind{}
	for _, d := range overrides {
		ov[leave.FormatDate(d.Date)] = d.Kind
	}
	return func(d time.Time) bool {
		if k, ok := ov[leave.FormatDate(d)]; ok {
			return k == KindHoliday
		}
		ok, _ := FederalHoliday(d)
		return ok
	}
}
