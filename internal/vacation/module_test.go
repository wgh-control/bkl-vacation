package vacation

import (
	"strings"
	"testing"

	"github.com/wgh-control/bkl-vacation/internal/config"
	"github.com/wgh-control/bkl-vacation/internal/store"
)

func TestTablesKeepLedgerNames(t *testing.T) {
	tables := Tables()
	if len(tables) != 2 {
		t.Fatalf("tables %d", len(tables))
	}
	if tables[0].Version != "001_init.sql" || tables[1].Version != "002_finance.sql" {
		t.Fatalf("versions %s %s", tables[0].Version, tables[1].Version)
	}
	if !strings.Contains(tables[0].SQL, "CREATE TABLE employee") || !strings.Contains(tables[0].SQL, "CREATE TABLE leave_request") {
		t.Fatal("shared org tables and leave tables belong to the vacation migration")
	}
	if !strings.Contains(tables[1].SQL, "finance_sent_at") {
		t.Fatal(tables[1].SQL)
	}
	m := New(store.NewMemory(), "hr", config.ClaimNames{}, nil, "")
	if m.Name() != "vacation" || m.Mail() == nil || len(m.Tables()) != 2 {
		t.Fatal("module")
	}
}
