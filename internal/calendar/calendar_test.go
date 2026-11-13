package calendar

import (
	"testing"
	"time"

	"github.com/wgh-control/bkl-vacation/internal/leave"
)

func TestFederalList(t *testing.T) {
	cases := []struct {
		d    time.Time
		code string
	}{
		{time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), "new_year"},
		{time.Date(2027, 1, 6, 0, 0, 0, 0, time.UTC), "new_year"},
		{time.Date(2027, 1, 7, 0, 0, 0, 0, time.UTC), "christmas"},
		{time.Date(2027, 1, 8, 0, 0, 0, 0, time.UTC), "new_year"},
		{time.Date(2027, 2, 23, 0, 0, 0, 0, time.UTC), "defender"},
		{time.Date(2027, 3, 8, 0, 0, 0, 0, time.UTC), "womens_day"},
		{time.Date(2027, 5, 1, 0, 0, 0, 0, time.UTC), "spring_labour"},
		{time.Date(2027, 5, 9, 0, 0, 0, 0, time.UTC), "victory"},
		{time.Date(2027, 6, 12, 0, 0, 0, 0, time.UTC), "russia_day"},
		{time.Date(2027, 11, 4, 0, 0, 0, 0, time.UTC), "unity"},
	}
	for _, tc := range cases {
		ok, code := FederalHoliday(tc.d)
		if !ok || code != tc.code {
			t.Fatalf("%s: %v %s", leave.FormatDate(tc.d), ok, code)
		}
	}
	if ok, _ := FederalHoliday(time.Date(2027, 1, 9, 0, 0, 0, 0, time.UTC)); ok {
		t.Fatal("9 January is not a federal holiday")
	}
}

func TestJanuaryWeekendIsNotAutoTransferred(t *testing.T) {
	// 1 January 2027 is a Friday, so 2 January is Saturday and a holiday.
	days := Build(
		time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2027, 1, 12, 0, 0, 0, 0, time.UTC),
		nil, "en",
	)
	by := index(days)
	if by["2027-01-02"].Kind != KindHoliday {
		t.Fatalf("2 Jan %s", by["2027-01-02"].Kind)
	}
	if by["2027-01-11"].Kind == KindTransferred {
		t.Fatal("January weekend holidays are not moved onto the next working day")
	}
}

func TestWeekendHolidayTransfersOutsideJanuary(t *testing.T) {
	// 12 June 2027 is a Saturday.
	days := Build(
		time.Date(2027, 6, 11, 0, 0, 0, 0, time.UTC),
		time.Date(2027, 6, 15, 0, 0, 0, 0, time.UTC),
		nil, "ru",
	)
	by := index(days)
	if by["2027-06-12"].Kind != KindHoliday {
		t.Fatalf("12 Jun %s", by["2027-06-12"].Kind)
	}
	if by["2027-06-14"].Kind != KindTransferred {
		t.Fatalf("14 Jun should be the transferred rest day, got %s", by["2027-06-14"].Kind)
	}
	if by["2027-06-14"].Name != "Перенесённый выходной" {
		t.Fatalf("name %q", by["2027-06-14"].Name)
	}
	// The transferred rest day still counts as a vacation day.
	h := HolidayFunc(nil)
	if h(time.Date(2027, 6, 14, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("a transferred weekend counts as a vacation day")
	}
	if !h(time.Date(2027, 6, 12, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("the holiday itself does not count")
	}
}

func TestOverrideWins(t *testing.T) {
	day := time.Date(2027, 6, 14, 0, 0, 0, 0, time.UTC)
	days := Build(
		time.Date(2027, 6, 12, 0, 0, 0, 0, time.UTC),
		time.Date(2027, 6, 14, 0, 0, 0, 0, time.UTC),
		[]Day{{Date: day, Kind: KindWork, Name: "Working Saturday transfer", Source: "government"}},
		"en",
	)
	got := days[len(days)-1]
	if got.Kind != KindWork || got.Source != "government" {
		t.Fatalf("override %+v", got)
	}
	h := HolidayFunc([]Day{{Date: time.Date(2027, 5, 1, 0, 0, 0, 0, time.UTC), Kind: KindWork, Source: "government"}})
	if h(time.Date(2027, 5, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("an override can mark a statutory holiday as a working day")
	}
	regional := time.Date(2027, 4, 12, 0, 0, 0, 0, time.UTC)
	h = HolidayFunc([]Day{{Date: regional, Kind: KindHoliday, Name: "Regional", Source: "regional"}})
	if !h(regional) {
		t.Fatal("a regional holiday is excluded from counted days")
	}
}

func index(days []Day) map[string]Day {
	out := map[string]Day{}
	for _, d := range days {
		out[leave.FormatDate(d.Date)] = d
	}
	return out
}
