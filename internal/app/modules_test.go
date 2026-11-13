package app

import (
	"testing"

	"github.com/wgh-control/bkl-vacation/internal/store"
	"github.com/wgh-control/bkl-vacation/internal/vacation"
)

func TestVacationIsRegistered(t *testing.T) {
	mods := Modules(Services{Repo: store.NewMemory(), HRGroup: "hr"})
	if len(mods) != 1 || mods[0].Name() != "vacation" || mods[0].Mail() == nil {
		t.Fatalf("modules %+v", mods)
	}
	if Vacation(mods) == nil {
		t.Fatal("vacation service")
	}
	tables := Tables()
	want := vacation.Tables()
	if len(tables) != len(want) {
		t.Fatalf("tables %d", len(tables))
	}
	for i := range want {
		if tables[i].Version != want[i].Version {
			t.Fatalf("version %s", tables[i].Version)
		}
	}
}
