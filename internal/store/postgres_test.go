package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wgh-control/bkl-vacation/internal/config"
	"github.com/wgh-control/bkl-vacation/internal/identity"
	"github.com/wgh-control/bkl-vacation/internal/portal"
	"github.com/wgh-control/bkl-vacation/internal/store"
	"github.com/wgh-control/bkl-vacation/migrations"
)

func TestPostgresRequestFlow(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
TRUNCATE portal_notice, notification, audit_event, approval_step, leave_request,
         leave_year, schedule_lock, employee, org_unit, calendar_day CASCADE`); err != nil {
		t.Fatal(err)
	}
	repo := store.NewPostgres(pool)
	svc := portal.New(repo, "hr", config.ClaimNames{
		HireDate: "hire_date", Manager: "manager", Department: "department", EmployeeNumber: "employee_number",
	})
	svc.Now = func() time.Time { return time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC) }
	mgr := identity.Principal{Subject: "mgr", Name: "Mia", Email: "mia@example.com", Claims: map[string]string{"hire_date": "2020-03-01", "department": "Engineering"}}
	hr := identity.Principal{Subject: "hr", Name: "Ira", Email: "ira@example.com", Groups: []string{"hr"}, Claims: map[string]string{"hire_date": "2020-03-01", "department": "HR", "manager": "mgr"}}
	emp := identity.Principal{Subject: "ada", Name: "Ada", Email: "ada@example.com", Claims: map[string]string{"hire_date": "2020-03-01", "department": "Engineering", "manager": "mgr", "employee_number": "42"}}
	for _, p := range []identity.Principal{mgr, hr, emp} {
		if _, err := svc.Me(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	req, err := svc.CreateRequest(ctx, emp, portal.CreateInput{Type: "vacation", StartsOn: "2027-06-15", EndsOn: "2027-06-28"})
	if err != nil {
		t.Fatal(err)
	}
	if req.CountedDays != 14 || req.WorkingYearStart != "2027-03-01" {
		t.Fatalf("%+v", req)
	}
	if _, err := svc.Decide(ctx, mgr, req.ID, portal.DecisionInput{Decision: "approve"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(ctx, hr, req.ID, portal.DecisionInput{Decision: "approve"}); err != nil {
		t.Fatal(err)
	}
	days, err := svc.YearDays(ctx, emp)
	if err != nil {
		t.Fatal(err)
	}
	var next *portal.YearDays
	for i := range days.Years {
		if days.Years[i].Relation == "next" {
			next = &days.Years[i]
		}
	}
	if next == nil || next.ApprovedDays != 14 || next.Year != 2027 {
		t.Fatalf("%+v", days.Years)
	}
	// HR uses the employee route for their own request and the queue for others.
	own, err := svc.CreateRequest(ctx, hr, portal.CreateInput{Type: "sick", StartsOn: "2026-11-01", EndsOn: "2026-11-02", CertificateNumber: "LN"})
	if err != nil {
		t.Fatal(err)
	}
	if own.EmployeeID == "" {
		t.Fatal("hr own request")
	}
	file, err := svc.Export(ctx, hr, "all", "", "", "xls", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Body) == 0 || file.Name != "bkl-leave.xls" {
		t.Fatalf("%s %d", file.Name, len(file.Body))
	}
	ok, err := repo.WithAdvisoryLock(ctx, func(context.Context) error { return nil })
	if err != nil || !ok {
		t.Fatalf("lock %v %v", ok, err)
	}
}
