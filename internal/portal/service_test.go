package portal

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wgh-control/bkl-vacation/internal/config"
	"github.com/wgh-control/bkl-vacation/internal/identity"
	"github.com/wgh-control/bkl-vacation/internal/model"
	"github.com/wgh-control/bkl-vacation/internal/notify"
	"github.com/wgh-control/bkl-vacation/internal/store"
)

func newSvc(t *testing.T) (*Service, *store.Memory) {
	t.Helper()
	mem := store.NewMemory()
	svc := New(mem, "hr", config.ClaimNames{
		Email: "email", Locale: "locale", EmployeeNumber: "employee_number",
		Position: "position", Department: "department", Manager: "manager", HireDate: "hire_date",
	})
	svc.Now = func() time.Time { return time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC) }
	return svc, mem
}

func person(sub, name, manager string, groups ...string) identity.Principal {
	return identity.Principal{
		Subject: sub, Name: name, Email: sub + "@example.com", Groups: groups,
		Claims: map[string]string{
			"hire_date": "2020-03-01", "manager": manager, "department": "Engineering",
			"employee_number": sub, "position": "Engineer",
		},
	}
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	pe, ok := err.(*Error)
	if !ok {
		t.Fatalf("error %v", err)
	}
	return pe.Code
}

func TestNextYearPlanningAndWorkingYearBalance(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	mgr := person("mgr", "Mia", "")
	emp := person("emp", "Ada", "mgr")
	if _, err := svc.Me(ctx, mgr); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Me(ctx, emp); err != nil {
		t.Fatal(err)
	}
	// June 2027 is the next calendar year and the next working year (hire date 1 March).
	next, err := svc.CreateRequest(ctx, emp, CreateInput{Type: "vacation", StartsOn: "2027-06-15", EndsOn: "2027-06-28"})
	if err != nil {
		t.Fatal(err)
	}
	if next.CountedDays != 14 || next.Status != "pending_manager" {
		t.Fatalf("%+v", next)
	}
	if next.WorkingYearStart != "2027-03-01" {
		t.Fatalf("working year %s", next.WorkingYearStart)
	}
	// January 2027 is next calendar year but still the working year that started in 2026.
	jan, err := svc.CreateRequest(ctx, emp, CreateInput{Type: "vacation", StartsOn: "2027-01-11", EndsOn: "2027-01-24"})
	if err != nil {
		t.Fatal(err)
	}
	if jan.WorkingYearStart != "2026-03-01" || jan.CountedDays != 14 {
		t.Fatalf("%+v", jan)
	}
	days, err := svc.YearDays(ctx, emp)
	if err != nil {
		t.Fatal(err)
	}
	var past, current, nextYear *YearDays
	for i := range days.Years {
		y := &days.Years[i]
		switch y.Relation {
		case "past":
			if y.Year == 2025 {
				past = y
			}
		case "current":
			current = y
		case "next":
			nextYear = y
		}
	}
	if past == nil || current == nil || nextYear == nil {
		t.Fatalf("missing buckets %#v", days.Years)
	}
	if current.Year != 2026 || nextYear.Year != 2027 {
		t.Fatalf("years %d %d", current.Year, nextYear.Year)
	}
	if nextYear.PlannedDays != 28 || nextYear.ApprovedDays != 0 {
		t.Fatalf("next planned %d approved %d", nextYear.PlannedDays, nextYear.ApprovedDays)
	}
	bal, err := svc.Balance(ctx, emp)
	if err != nil {
		t.Fatal(err)
	}
	var y2026, y2027 *WorkingYearView
	for i := range bal.WorkingYears {
		w := &bal.WorkingYears[i]
		if w.StartsOn == "2026-03-01" {
			y2026 = w
		}
		if w.StartsOn == "2027-03-01" {
			y2027 = w
		}
	}
	if y2026 == nil || y2026.ReservedDays != 14 || y2026.GrantedDays != 28 {
		t.Fatalf("2026 balance %+v", y2026)
	}
	if y2027 == nil || y2027.ReservedDays != 14 || y2027.FreeDays != 14 {
		t.Fatalf("2027 balance %+v", y2027)
	}
	if _, err := svc.CreateRequest(ctx, emp, CreateInput{Type: "vacation", StartsOn: "2028-01-10", EndsOn: "2028-01-20"}); codeOf(t, err) != "planning_horizon" {
		t.Fatal(err)
	}
}

func TestStreakHolidaysAndApprovals(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	mgr := person("mgr", "Mia", "")
	hr := person("hr", "Ira", "mgr", "hr")
	emp := person("emp", "Ada", "mgr")
	for _, p := range []identity.Principal{mgr, hr, emp} {
		if _, err := svc.Me(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	// 10 days leaves a 14-day part possible. A second 10 does not.
	first, err := svc.CreateRequest(ctx, emp, CreateInput{Type: "vacation", StartsOn: "2026-11-16", EndsOn: "2026-11-25"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateRequest(ctx, emp, CreateInput{Type: "vacation", StartsOn: "2026-12-01", EndsOn: "2026-12-10"}); codeOf(t, err) != "streak_impossible" {
		t.Fatal(err)
	}
	if _, err := svc.Cancel(ctx, emp, first.ID); err != nil {
		t.Fatal(err)
	}
	// 14 counted days that include a weekend.
	vac, err := svc.CreateRequest(ctx, emp, CreateInput{Type: "vacation", StartsOn: "2026-11-16", EndsOn: "2026-11-29"})
	if err != nil || vac.CountedDays != 14 {
		t.Fatalf("%v %+v", err, vac)
	}
	soon, err := svc.CreateRequest(ctx, emp, CreateInput{Type: "vacation", StartsOn: "2026-11-16", EndsOn: "2026-11-20"})
	if err == nil {
		t.Fatal("overlap should be rejected")
	}
	_ = soon
	if codeOf(t, err) != "overlap" {
		t.Fatal(err)
	}
	if _, err := svc.Decide(ctx, mgr, vac.ID, DecisionInput{Decision: "approve"}); err != nil {
		t.Fatal(err)
	}
	approved, err := svc.Decide(ctx, hr, vac.ID, DecisionInput{Decision: "approve"})
	if err != nil {
		t.Fatal(err)
	}
	if approved.Status != "awaiting_confirmation" || !approved.PayByWarning {
		t.Fatalf("%+v", approved)
	}
	days, err := svc.YearDays(ctx, emp)
	if err != nil {
		t.Fatal(err)
	}
	for _, y := range days.Years {
		if y.Year == 2026 && y.ApprovedDays != 14 {
			t.Fatalf("approved %d", y.ApprovedDays)
		}
	}
	moved, err := svc.Reschedule(ctx, emp, vac.ID, CreateInput{StartsOn: "2027-06-15", EndsOn: "2027-06-28"})
	if err != nil {
		t.Fatal(err)
	}
	if moved.ReplacesRequestID != vac.ID || moved.Status != "pending_manager" {
		t.Fatalf("%+v", moved)
	}
	if _, err := svc.Decide(ctx, mgr, moved.ID, DecisionInput{Decision: "approve"}); err != nil {
		t.Fatal(err)
	}
	done, err := svc.Decide(ctx, hr, moved.ID, DecisionInput{Decision: "approve"})
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != "scheduled" {
		t.Fatalf("status %s", done.Status)
	}
	days, err = svc.YearDays(ctx, emp)
	if err != nil {
		t.Fatal(err)
	}
	for _, y := range days.Years {
		if y.Year == 2026 && (y.ApprovedDays != 0 || y.MovedAwayDays != 14) {
			t.Fatalf("2026 %+v", y)
		}
		if y.Year == 2027 && (y.ApprovedDays != 14 || y.MovedDays != 14) {
			t.Fatalf("2027 %+v", y)
		}
	}
	// The original dates stayed until HR approved; a rejected move would have kept them.
	old, err := svc.GetRequest(ctx, emp, vac.ID)
	if err != nil || old.Status != "cancelled" || old.CancelReason != "rescheduled" {
		t.Fatalf("%v %+v", err, old)
	}
}

func TestHRIsAlsoAnEmployee(t *testing.T) {
	svc, mem := newSvc(t)
	ctx := context.Background()
	mgr := person("mgr", "Mia", "")
	hr := person("hr", "Ira", "mgr", "hr")
	other := person("other-hr", "Noa", "mgr", "hr")
	for _, p := range []identity.Principal{mgr, hr, other} {
		if _, err := svc.Me(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	vac, err := svc.CreateRequest(ctx, hr, CreateInput{Type: "vacation", StartsOn: "2027-06-15", EndsOn: "2027-06-28"})
	if err != nil {
		t.Fatal(err)
	}
	off, err := svc.CreateRequest(ctx, hr, CreateInput{Type: "day_off", StartsOn: "2026-12-14", EndsOn: "2026-12-14", ReasonCode: "compensatory_overtime"})
	if err != nil {
		t.Fatal(err)
	}
	sick, err := svc.CreateRequest(ctx, hr, CreateInput{Type: "sick", StartsOn: "2026-11-02", EndsOn: "2026-11-03", CertificateNumber: "LN-1"})
	if err != nil {
		t.Fatal(err)
	}
	mine, err := svc.ListMine(ctx, hr)
	if err != nil || len(mine) != 3 {
		t.Fatalf("%v %d", err, len(mine))
	}
	if _, err := svc.Decide(ctx, mgr, vac.ID, DecisionInput{Decision: "approve"}); err != nil {
		t.Fatal(err)
	}
	queue, err := svc.HRQueue(ctx, hr)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range queue {
		if item.ID == vac.ID {
			found = true
			if item.ManagerDecision == nil || item.ManagerDecision.Decision != "approve" {
				t.Fatalf("manager decision %+v", item.ManagerDecision)
			}
		}
	}
	if !found {
		t.Fatal("HR queue missing own vacation")
	}
	// HR may approve their own request after the manager did.
	if _, err := svc.Decide(ctx, hr, vac.ID, DecisionInput{Decision: "approve"}); err != nil {
		t.Fatal(err)
	}
	_ = off
	_ = sick
	// Self-approval when the manager is themselves is refused.
	// The token has no manager claim here, so the stored self-link is kept.
	if err := mem.RunInTx(ctx, func(q store.Queries) error {
		e, err := q.GetEmployeeBySubject(ctx, "hr")
		if err != nil {
			return err
		}
		e.ManagerID = e.ID
		return q.UpdateEmployee(ctx, e)
	}); err != nil {
		t.Fatal(err)
	}
	selfPrincipal := hr
	selfPrincipal.Claims = map[string]string{"hire_date": "2020-03-01", "department": "Engineering", "employee_number": "hr"}
	selfReq, err := svc.CreateRequest(ctx, selfPrincipal, CreateInput{Type: "day_off", StartsOn: "2026-12-21", EndsOn: "2026-12-21", ReasonCode: "unpaid_personal"})
	if err != nil {
		t.Fatal(err)
	}
	if selfReq.Status != "pending_hr" || !selfReq.SelfApproval {
		t.Fatalf("%+v", selfReq)
	}
	if codeOf(t, mustErr(svc.Decide(ctx, selfPrincipal, selfReq.ID, DecisionInput{Decision: "approve"}))) != "self_approval" {
		t.Fatal("same HR person must not clear a skipped manager step")
	}
	if _, err := svc.Decide(ctx, other, selfReq.ID, DecisionInput{Decision: "approve"}); err != nil {
		t.Fatal(err)
	}
}

func TestExportScopesAndFormats(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	mgr := person("mgr", "Mia", "")
	hr := person("hr", "Ira", "", "hr")
	hr.Claims["department"] = "HR"
	a := person("a", "Ada", "mgr")
	a.Claims["department"] = "Engineering"
	b := person("b", "Bea", "mgr")
	b.Claims["department"] = "Finance"
	for _, p := range []identity.Principal{mgr, hr, a, b} {
		if _, err := svc.Me(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	ra, err := svc.CreateRequest(ctx, a, CreateInput{Type: "vacation", StartsOn: "2027-06-15", EndsOn: "2027-06-28"})
	if err != nil {
		t.Fatal(err)
	}
	rb, err := svc.CreateRequest(ctx, b, CreateInput{Type: "vacation", StartsOn: "2027-07-05", EndsOn: "2027-07-18"})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{ra.ID, rb.ID} {
		if _, err := svc.Decide(ctx, mgr, id, DecisionInput{Decision: "approve"}); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Decide(ctx, hr, id, DecisionInput{Decision: "approve"}); err != nil {
			t.Fatal(err)
		}
	}
	pa, err := svc.Me(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	all, err := svc.Export(ctx, hr, "all", "", "", "csv", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !bytesContains(all.Body, "Ada") || !bytesContains(all.Body, "Bea") || !bytesContains(all.Body, "Сотрудник") {
		t.Fatalf("csv %s", all.Body)
	}
	one, err := svc.Export(ctx, hr, "user", pa.ID, "", "xls", "2027-01-01", "2027-12-31")
	if err != nil {
		t.Fatal(err)
	}
	if !bytesContains(one.Body, "Ada") || bytesContains(one.Body, "Bea") {
		t.Fatalf("user scope %s", one.Body)
	}
	dept, err := svc.Export(ctx, hr, "department", "", pa.Department.ID, "xlsx", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(dept.Body) < 4 || string(dept.Body[:2]) != "PK" {
		t.Fatal("xlsx")
	}
	pdf, err := svc.Export(ctx, hr, "all", "", "", "pdf", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if string(pdf.Body[:4]) != "%PDF" {
		t.Fatal("pdf")
	}
	if _, err := svc.Export(ctx, a, "all", "", "", "csv", "", ""); codeOf(t, err) != "forbidden" {
		t.Fatal(err)
	}
}

func TestSickOverlapAndUnpaidExcess(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	mgr := person("mgr", "Mia", "")
	hr := person("hr", "Ira", "mgr", "hr")
	emp := person("emp", "Ada", "mgr")
	for _, p := range []identity.Principal{mgr, hr, emp} {
		if _, err := svc.Me(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	long, err := svc.CreateRequest(ctx, emp, CreateInput{Type: "vacation", StartsOn: "2027-06-15", EndsOn: "2027-07-04"})
	if err != nil || long.CountedDays != 20 {
		t.Fatalf("%v %+v", err, long)
	}
	short, err := svc.CreateRequest(ctx, emp, CreateInput{Type: "vacation", StartsOn: "2027-08-02", EndsOn: "2027-08-09"})
	if err != nil || short.CountedDays != 8 {
		t.Fatalf("%v %+v", err, short)
	}
	for _, id := range []string{long.ID, short.ID} {
		if _, err := svc.Decide(ctx, mgr, id, DecisionInput{Decision: "approve"}); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Decide(ctx, hr, id, DecisionInput{Decision: "approve"}); err != nil {
			t.Fatal(err)
		}
	}
	bare, err := svc.CreateRequest(ctx, emp, CreateInput{Type: "sick", StartsOn: "2027-06-20", EndsOn: "2027-06-26"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(ctx, mgr, bare.ID, DecisionInput{Decision: "approve"}); err != nil {
		t.Fatal(err)
	}
	if codeOf(t, mustErr(svc.Decide(ctx, hr, bare.ID, DecisionInput{Decision: "approve"}))) != "certificate_required" {
		t.Fatal("certificate")
	}
	if _, err := svc.SetCertificate(ctx, emp, bare.ID, "LN-99"); err != nil {
		t.Fatal(err)
	}
	if codeOf(t, mustErr(svc.Decide(ctx, hr, bare.ID, DecisionInput{Decision: "approve"}))) != "overlap_choice_required" {
		t.Fatal("choice")
	}
	if _, err := svc.SetOverlapChoice(ctx, emp, bare.ID, "postpone"); err != nil {
		t.Fatal(err)
	}
	if codeOf(t, mustErr(svc.Decide(ctx, hr, bare.ID, DecisionInput{Decision: "approve"}))) != "streak_impossible" {
		t.Fatal("postpone must keep a 14-day part possible")
	}
	if _, err := svc.SetOverlapChoice(ctx, emp, bare.ID, "extend"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(ctx, hr, bare.ID, DecisionInput{Decision: "approve"}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetRequest(ctx, emp, long.ID)
	if err != nil || got.CountedDays != 20 || got.EndsOn == "2027-07-04" {
		t.Fatalf("extended %+v", got)
	}
	off, err := svc.CreateRequest(ctx, emp, CreateInput{Type: "day_off", StartsOn: "2026-12-01", EndsOn: "2026-12-20", ReasonCode: "unpaid_personal"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(ctx, mgr, off.ID, DecisionInput{Decision: "approve"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(ctx, hr, off.ID, DecisionInput{Decision: "approve"}); err != nil {
		t.Fatal(err)
	}
	bal, err := svc.Balance(ctx, emp)
	if err != nil {
		t.Fatal(err)
	}
	for _, y := range bal.WorkingYears {
		if y.StartsOn == "2026-03-01" {
			if y.UnpaidExcessDays != 6 || y.AccruedAdjustment >= 0 {
				t.Fatalf("article 121 %+v", y)
			}
		}
	}
	family, err := svc.CreateRequest(ctx, emp, CreateInput{Type: "day_off", StartsOn: "2026-12-21", EndsOn: "2026-12-23", ReasonCode: "unpaid_family"})
	if err != nil {
		t.Fatal(err)
	}
	if codeOf(t, mustErr(svc.Decide(ctx, mgr, family.ID, DecisionInput{Decision: "reject"}))) != "reason_mandatory" {
		t.Fatal("family leave")
	}
}

func TestReserveIsSingleSpender(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	emp := person("emp", "Ada", "")
	if _, err := svc.Me(ctx, emp); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		start := "2027-06-15"
		end := "2027-07-04"
		if i == 1 {
			start, end = "2027-08-02", "2027-08-21"
		}
		go func(start, end string) {
			defer wg.Done()
			_, err := svc.CreateRequest(ctx, emp, CreateInput{Type: "vacation", StartsOn: start, EndsOn: end})
			errs <- err
		}(start, end)
	}
	wg.Wait()
	close(errs)
	ok, bad := 0, 0
	for err := range errs {
		if err == nil {
			ok++
			continue
		}
		if codeOf(t, err) == "balance" || codeOf(t, err) == "streak_impossible" {
			bad++
			continue
		}
		t.Fatal(err)
	}
	if ok != 1 || bad != 1 {
		t.Fatalf("ok %d bad %d", ok, bad)
	}
}

func TestDispatchCreatesNoticeWhenMailFails(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	mgr := person("mgr", "Mia", "")
	hr := person("hr", "Ira", "mgr", "hr")
	emp := person("emp", "Ada", "mgr")
	for _, p := range []identity.Principal{mgr, hr, emp} {
		if _, err := svc.Me(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	vac, err := svc.CreateRequest(ctx, emp, CreateInput{Type: "vacation", StartsOn: "2026-11-16", EndsOn: "2026-11-29"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(ctx, mgr, vac.ID, DecisionInput{Decision: "approve"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(ctx, hr, vac.ID, DecisionInput{Decision: "approve"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Dispatch(ctx, failMail{}); err != nil {
		t.Fatal(err)
	}
	notes, err := svc.Notices(ctx, emp)
	if err != nil || len(notes) == 0 {
		t.Fatalf("portal notice %v %d", err, len(notes))
	}
	if err := svc.Dispatch(ctx, failMail{}); err != nil {
		t.Fatal(err)
	}
	again, err := svc.Notices(ctx, emp)
	if err != nil || len(again) != len(notes) {
		t.Fatalf("duplicate notices %d %d", len(notes), len(again))
	}
}

func TestDirectorySyncLinksManager(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	hire := time.Date(2019, 1, 2, 0, 0, 0, 0, time.UTC)
	err := svc.ApplyDirectory(ctx, []DirectoryUser{
		{Subject: "m", DisplayName: "Mia", Email: "mia@example.com", Active: true, Department: "Engineering"},
		{Subject: "e", DisplayName: "Ada", Email: "ada@example.com", Active: true, Department: "Engineering/Platform", ManagerRaw: "m", HireDate: &hire, EmployeeNumber: "42"},
		{Subject: "old", DisplayName: "Gone", Active: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	var ada model.Employee
	if err := svc.Repo.View(ctx, func(q store.Queries) error {
		e, err := q.GetEmployeeBySubject(ctx, "e")
		ada = e
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if ada.ManagerID == "" || ada.EmployeeNumber != "42" || !ada.Active {
		t.Fatalf("%+v", ada)
	}
	var gone model.Employee
	if err := svc.Repo.View(ctx, func(q store.Queries) error {
		e, err := q.GetEmployeeBySubject(ctx, "old")
		gone = e
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if gone.Active {
		t.Fatal("disabled accounts stay, marked inactive")
	}
}

type failMail struct{}

func (failMail) Send(context.Context, notify.Message) error {
	return notify.ErrNotConfigured
}

type captureMail struct {
	msgs []notify.Message
}

func (c *captureMail) Send(_ context.Context, msg notify.Message) error {
	c.msgs = append(c.msgs, msg)
	return nil
}

func TestWorkflowMail(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	mgr := person("mgr", "Mia", "")
	hr := person("hr", "Ira", "mgr", "hr")
	emp := person("emp", "Ada", "mgr")
	for _, p := range []identity.Principal{mgr, hr, emp} {
		if _, err := svc.Me(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	req, err := svc.CreateRequest(ctx, emp, CreateInput{Type: "vacation", StartsOn: "2027-06-15", EndsOn: "2027-06-28"})
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	if err := svc.Repo.View(ctx, func(q store.Queries) error {
		due, err := q.ListUnsentDue(ctx, svc.Now())
		for _, n := range due {
			if n.RequestID == req.ID {
				kinds = append(kinds, n.Kind)
			}
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !hasKind(kinds, "submitted") || !hasKind(kinds, "approval_needed_manager") || hasKind(kinds, "approval_needed_hr") {
		t.Fatalf("%v", kinds)
	}
	off, err := svc.CreateRequest(ctx, emp, CreateInput{Type: "day_off", StartsOn: "2026-12-14", EndsOn: "2026-12-14", ReasonCode: "compensatory_overtime"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(ctx, mgr, off.ID, DecisionInput{Decision: "reject", Comment: "no"}); err != nil {
		t.Fatal(err)
	}
	box := &captureMail{}
	if err := svc.Dispatch(ctx, box); err != nil {
		t.Fatal(err)
	}
	var declined notify.Message
	for _, msg := range box.msgs {
		if strings.Contains(msg.Subject, "отклонена") {
			declined = msg
		}
	}
	if declined.HTML == "" || !strings.Contains(declined.HTML, "#f4f1eb") || !strings.Contains(declined.Body, "отгул") {
		t.Fatalf("%+v", declined)
	}
}

func TestSendApprovedVacationToFinance(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	mgr := person("mgr", "Mia", "")
	hr := person("hr", "Ira", "mgr", "hr")
	emp := person("emp", "Ada", "mgr")
	for _, p := range []identity.Principal{mgr, hr, emp} {
		if _, err := svc.Me(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	vac, err := svc.CreateRequest(ctx, emp, CreateInput{Type: "vacation", StartsOn: "2027-06-15", EndsOn: "2027-06-28"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(ctx, mgr, vac.ID, DecisionInput{Decision: "approve"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(ctx, hr, vac.ID, DecisionInput{Decision: "approve"}); err != nil {
		t.Fatal(err)
	}
	var kinds []string
	if err := svc.Repo.View(ctx, func(q store.Queries) error {
		due, err := q.ListUnsentDue(ctx, svc.Now())
		for _, n := range due {
			if n.RequestID == vac.ID {
				kinds = append(kinds, n.Kind)
			}
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !hasKind(kinds, "approved_hr") {
		t.Fatalf("HR did not receive the approved vacation: %v", kinds)
	}
	if _, err := svc.SendToFinance(ctx, hr, vac.ID); codeOf(t, err) != "finance_unconfigured" {
		t.Fatal(err)
	}
	box := &captureMail{}
	svc.Mail = box
	svc.FinanceEmail = "payroll@example.com"
	sent, err := svc.SendToFinance(ctx, hr, vac.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sent.FinanceSentAt == "" || len(box.msgs) != 1 || box.msgs[0].To != "payroll@example.com" {
		t.Fatalf("%+v %d", sent, len(box.msgs))
	}
	if !strings.Contains(box.msgs[0].HTML, "Финансам") || !strings.Contains(box.msgs[0].Body, "emp") || !strings.Contains(box.msgs[0].HTML, "#f4f1eb") {
		t.Fatal(box.msgs[0].Body)
	}
	if codeOf(t, mustErr(svc.SendToFinance(ctx, hr, vac.ID))) != "already_sent" {
		t.Fatal("second send")
	}
	off, err := svc.CreateRequest(ctx, emp, CreateInput{Type: "day_off", StartsOn: "2026-12-14", EndsOn: "2026-12-14", ReasonCode: "compensatory_overtime"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(ctx, mgr, off.ID, DecisionInput{Decision: "approve"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(ctx, hr, off.ID, DecisionInput{Decision: "approve"}); err != nil {
		t.Fatal(err)
	}
	if codeOf(t, mustErr(svc.SendToFinance(ctx, hr, off.ID))) != "invalid_status" {
		t.Fatal("day off")
	}
}

func TestDirectoryManagerChangeWithoutOverride(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	if err := svc.ApplyDirectory(ctx, []DirectoryUser{
		{Subject: "mia", DisplayName: "Mia", Email: "mia@example.com", Active: true},
		{Subject: "ada", DisplayName: "Ada", Email: "ada@example.com", Active: true, ManagerRaw: "mia"},
	}); err != nil {
		t.Fatal(err)
	}
	ada := employeeBySubject(t, svc, "ada")
	if ada.ManagerID == "" || ada.ADManagerID != ada.ManagerID || ada.ManagerOverrideID != "" {
		t.Fatalf("%+v", ada)
	}
	if err := svc.ApplyDirectory(ctx, []DirectoryUser{
		{Subject: "nova", DisplayName: "Nova", Email: "nova@example.com", Active: true},
		{Subject: "ada", DisplayName: "Ada", Email: "ada@example.com", Active: true, ManagerRaw: "nova"},
	}); err != nil {
		t.Fatal(err)
	}
	ada = employeeBySubject(t, svc, "ada")
	nova := employeeBySubject(t, svc, "nova")
	if ada.ManagerID != nova.ID || ada.ADManagerID != nova.ID {
		t.Fatalf("shown manager %+v nova %s", ada, nova.ID)
	}
	if n := countKind(t, svc, notify.KindManagerADChanged); n != 0 {
		t.Fatalf("mail %d", n)
	}
	profile, err := svc.Me(ctx, person("ada", "Ada", "nova"))
	if err != nil || profile.Manager == nil || profile.Manager.Name != "Nova" {
		t.Fatalf("%+v %v", profile.Manager, err)
	}
}

func TestManualManagerOutweighsDirectory(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	if err := svc.ApplyDirectory(ctx, []DirectoryUser{
		{Subject: "mia", DisplayName: "Mia", Email: "mia@example.com", Active: true},
		{Subject: "nova", DisplayName: "Nova", Email: "nova@example.com", Active: true},
		{Subject: "kira", DisplayName: "Kira", Email: "kira@example.com", Active: true},
		{Subject: "ada", DisplayName: "Ada", Email: "ada@example.com", Active: true, ManagerRaw: "mia"},
		{Subject: "ira", DisplayName: "Ira", Email: "ira@example.com", Active: true, HR: true, Locale: "ru"},
		{Subject: "max", DisplayName: "Max", Email: "max@example.com", Active: true, HR: true, Locale: "en"},
	}); err != nil {
		t.Fatal(err)
	}
	ada := employeeBySubject(t, svc, "ada")
	kira := employeeBySubject(t, svc, "kira")
	mia := employeeBySubject(t, svc, "mia")
	ira := person("ira", "Ira", "", "hr")
	dir, err := svc.SetManager(ctx, ira, ada.ID, ManagerAction{Action: "set", ManagerID: kira.ID})
	if err != nil {
		t.Fatal(err)
	}
	adaRow := findManaged(t, dir, ada.ID)
	if !adaRow.Manual || adaRow.Manager == nil || adaRow.Manager.ID != kira.ID {
		t.Fatalf("%+v", adaRow)
	}
	if err := svc.ApplyDirectory(ctx, []DirectoryUser{
		{Subject: "ada", DisplayName: "Ada", Email: "ada@example.com", Active: true, ManagerRaw: "nova"},
	}); err != nil {
		t.Fatal(err)
	}
	ada = employeeBySubject(t, svc, "ada")
	nova := employeeBySubject(t, svc, "nova")
	if ada.ManagerID != kira.ID || ada.ManagerOverrideID != kira.ID || ada.ADManagerID != nova.ID || ada.ManagerConflictADID != nova.ID {
		t.Fatalf("%+v", ada)
	}
	if n := countKind(t, svc, notify.KindManagerADChanged); n != 2 {
		t.Fatalf("hr mail %d", n)
	}
	box := &captureMail{}
	if err := svc.Dispatch(ctx, box); err != nil {
		t.Fatal(err)
	}
	if len(box.msgs) != 2 {
		t.Fatalf("sent %d", len(box.msgs))
	}
	var ru, en notify.Message
	for _, msg := range box.msgs {
		if strings.Contains(msg.Subject, "Каталог") {
			ru = msg
		}
		if strings.Contains(msg.Subject, "Directory") {
			en = msg
		}
	}
	if !strings.Contains(ru.HTML, "Кадрам") || !strings.Contains(ru.HTML, "#d71015") || !strings.Contains(ru.Body, "Kira") || !strings.Contains(ru.HTML, "принять значение из каталога") {
		t.Fatalf("ru %s", ru.Body)
	}
	if !strings.Contains(en.HTML, "HR") || !strings.Contains(en.HTML, "#fdc000") || !strings.Contains(en.Body, "manual manager") || !strings.Contains(en.HTML, "keep the current manager") {
		t.Fatalf("en %s", en.Body)
	}
	if _, err := svc.SetManager(ctx, ira, ada.ID, ManagerAction{Action: "keep"}); err != nil {
		t.Fatal(err)
	}
	ada = employeeBySubject(t, svc, "ada")
	if ada.ManagerOverrideID != kira.ID || ada.ManagerConflictADID != "" || ada.ManagerID != kira.ID {
		t.Fatalf("kept %+v", ada)
	}
	if err := svc.ApplyDirectory(ctx, []DirectoryUser{
		{Subject: "ada", DisplayName: "Ada", Email: "ada@example.com", Active: true, ManagerRaw: "nova"},
	}); err != nil {
		t.Fatal(err)
	}
	if n := countKind(t, svc, notify.KindManagerADChanged); n != 0 {
		t.Fatalf("repeat mail %d", n)
	}
	if err := svc.ApplyDirectory(ctx, []DirectoryUser{
		{Subject: "ada", DisplayName: "Ada", Email: "ada@example.com", Active: true, ManagerRaw: "mia"},
	}); err != nil {
		t.Fatal(err)
	}
	ada = employeeBySubject(t, svc, "ada")
	if ada.ManagerID != kira.ID || ada.ADManagerID != mia.ID || ada.ManagerConflictADID != mia.ID {
		t.Fatalf("second change %+v", ada)
	}
	if n := countKind(t, svc, notify.KindManagerADChanged); n != 2 {
		t.Fatalf("second mail %d", n)
	}
	if _, err := svc.SetManager(ctx, ira, ada.ID, ManagerAction{Action: "take_ad"}); err != nil {
		t.Fatal(err)
	}
	ada = employeeBySubject(t, svc, "ada")
	if ada.ManagerOverrideID != "" || ada.ManagerID != mia.ID || ada.ManagerConflictADID != "" {
		t.Fatalf("taken %+v", ada)
	}
	profile, err := svc.Me(ctx, person("ada", "Ada", "mia"))
	if err != nil || profile.Manager == nil || profile.Manager.ID != mia.ID {
		t.Fatalf("profile %+v %v", profile.Manager, err)
	}
}

func TestApprovalUsesManualManager(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	if err := svc.ApplyDirectory(ctx, []DirectoryUser{
		{Subject: "mia", DisplayName: "Mia", Email: "mia@example.com", Active: true},
		{Subject: "kira", DisplayName: "Kira", Email: "kira@example.com", Active: true},
		{Subject: "ada", DisplayName: "Ada", Email: "ada@example.com", Active: true, ManagerRaw: "mia", HireDate: hireDate()},
		{Subject: "ira", DisplayName: "Ira", Email: "ira@example.com", Active: true, HR: true},
	}); err != nil {
		t.Fatal(err)
	}
	ada := employeeBySubject(t, svc, "ada")
	kira := employeeBySubject(t, svc, "kira")
	if _, err := svc.SetManager(ctx, person("ira", "Ira", "", "hr"), ada.ID, ManagerAction{Action: "set", ManagerID: kira.ID}); err != nil {
		t.Fatal(err)
	}
	req, err := svc.CreateRequest(ctx, person("ada", "Ada", "mia"), CreateInput{Type: "vacation", StartsOn: "2027-06-15", EndsOn: "2027-06-28"})
	if err != nil {
		t.Fatal(err)
	}
	if req.Status != string(model.StatusPendingManager) {
		t.Fatalf("%s", req.Status)
	}
	if codeOf(t, mustErr(svc.Decide(ctx, person("mia", "Mia", ""), req.ID, DecisionInput{Decision: "approve"}))) != "forbidden" {
		t.Fatal("directory manager must not approve over the manual value")
	}
	queue, err := svc.ListApprovals(ctx, person("kira", "Kira", ""))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range queue {
		if item.ID == req.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("manual manager queue")
	}
	if _, err := svc.Decide(ctx, person("kira", "Kira", ""), req.ID, DecisionInput{Decision: "approve"}); err != nil {
		t.Fatal(err)
	}
}

func hireDate() *time.Time {
	t := time.Date(2020, 3, 1, 0, 0, 0, 0, time.UTC)
	return &t
}

func employeeBySubject(t *testing.T, svc *Service, subject string) model.Employee {
	t.Helper()
	var emp model.Employee
	if err := svc.Repo.View(context.Background(), func(q store.Queries) error {
		e, err := q.GetEmployeeBySubject(context.Background(), subject)
		emp = e
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return emp
}

func findManaged(t *testing.T, dir ManagerDirectory, id string) ManagedEmployee {
	t.Helper()
	for _, group := range dir.Groups {
		for _, person := range group.People {
			if person.ID == id {
				return person
			}
		}
	}
	t.Fatalf("missing %s", id)
	return ManagedEmployee{}
}

func countKind(t *testing.T, svc *Service, kind string) int {
	t.Helper()
	var n int
	if err := svc.Repo.View(context.Background(), func(q store.Queries) error {
		due, err := q.ListUnsentDue(context.Background(), svc.Now().Add(time.Hour))
		if err != nil {
			return err
		}
		for _, item := range due {
			if item.Kind == kind {
				n++
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return n
}

func hasKind(kinds []string, want string) bool {
	for _, k := range kinds {
		if k == want {
			return true
		}
	}
	return false
}

func mustErr(v any, err error) error { return err }

func bytesContains(b []byte, s string) bool {
	return len(b) >= len(s) && (string(b) == s || indexBytes(b, s) >= 0)
}

func indexBytes(b []byte, s string) int {
	for i := 0; i+len(s) <= len(b); i++ {
		if string(b[i:i+len(s)]) == s {
			return i
		}
	}
	return -1
}
