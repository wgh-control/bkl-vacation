package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wgh-control/bkl-vacation/api"
	"github.com/wgh-control/bkl-vacation/internal/config"
	"github.com/wgh-control/bkl-vacation/internal/identity"
	"github.com/wgh-control/bkl-vacation/internal/notify"
	"github.com/wgh-control/bkl-vacation/internal/portal"
	"github.com/wgh-control/bkl-vacation/internal/store"
)

type headerAuth struct{}

func (headerAuth) Authenticate(r *http.Request) (identity.Principal, error) {
	sub := r.Header.Get("X-Sub")
	if sub == "" {
		return identity.Principal{}, errNoSub
	}
	p := identity.Principal{
		Subject: sub, Name: r.Header.Get("X-Name"), Email: sub + "@example.com",
		Claims: map[string]string{
			"hire_date": "2020-03-01", "manager": r.Header.Get("X-Manager"),
			"department": r.Header.Get("X-Dept"), "employee_number": sub,
		},
	}
	if g := r.Header.Get("X-Groups"); g != "" {
		p.Groups = strings.Split(g, ",")
	}
	return p, nil
}

var errNoSub = jsonError("missing")

type jsonError string

func (e jsonError) Error() string { return string(e) }

func TestEmployeeAndHRRoutes(t *testing.T) {
	mem := store.NewMemory()
	svc := portal.New(mem, "hr", config.ClaimNames{
		HireDate: "hire_date", Manager: "manager", Department: "department", EmployeeNumber: "employee_number",
	})
	svc.Now = func() time.Time { return time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC) }
	box := &sentMail{}
	svc.Mail = box
	svc.FinanceEmail = "payroll@example.com"
	srv := &Server{Svc: svc, Auth: headerAuth{}, Ready: mem.Ping}
	h := srv.Handler()

	do(t, h, http.MethodGet, "/api/healthz", "", nil, http.StatusOK)
	rec := do(t, h, http.MethodGet, "/api/me", "", map[string]string{"X-Sub": "mgr", "X-Name": "Mia"}, http.StatusOK)
	if !strings.Contains(rec, `"hr":false`) {
		t.Fatal(rec)
	}
	do(t, h, http.MethodGet, "/api/me", "", map[string]string{"X-Sub": "hr", "X-Name": "Ira", "X-Groups": "hr", "X-Dept": "HR"}, http.StatusOK)
	do(t, h, http.MethodGet, "/api/me", "", map[string]string{"X-Sub": "ada", "X-Name": "Ada", "X-Manager": "mgr", "X-Dept": "Engineering"}, http.StatusOK)

	body := `{"type":"vacation","starts_on":"2027-06-15","ends_on":"2027-06-28"}`
	rec = do(t, h, http.MethodPost, "/api/requests", body, map[string]string{"X-Sub": "ada", "X-Name": "Ada", "X-Manager": "mgr"}, http.StatusCreated)
	var created struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(rec), &created); err != nil || created.Status != "pending_manager" {
		t.Fatal(rec)
	}
	rec = do(t, h, http.MethodGet, "/api/me/days", "", map[string]string{"X-Sub": "ada", "X-Name": "Ada"}, http.StatusOK)
	if !strings.Contains(rec, `"relation":"next"`) || !strings.Contains(rec, `"planned_days":14`) {
		t.Fatal(rec)
	}
	do(t, h, http.MethodPost, "/api/approvals/"+created.ID+"/decision", `{"decision":"approve"}`, map[string]string{"X-Sub": "mgr", "X-Name": "Mia"}, http.StatusOK)
	do(t, h, http.MethodPost, "/api/approvals/"+created.ID+"/decision", `{"decision":"approve"}`, map[string]string{"X-Sub": "hr", "X-Name": "Ira", "X-Groups": "hr"}, http.StatusOK)
	rec = do(t, h, http.MethodPost, "/api/hr/requests/"+created.ID+"/finance", "", map[string]string{"X-Sub": "hr", "X-Name": "Ira", "X-Groups": "hr"}, http.StatusOK)
	if !strings.Contains(rec, `"finance_sent_at"`) || box.to != "payroll@example.com" || !strings.Contains(box.html, "Финансам") {
		t.Fatalf("finance %s to %s", rec, box.to)
	}
	do(t, h, http.MethodPost, "/api/hr/requests/"+created.ID+"/finance", "", map[string]string{"X-Sub": "hr", "X-Name": "Ira", "X-Groups": "hr"}, http.StatusConflict)

	// HR submits their own three request types on the employee routes.
	for _, raw := range []string{
		`{"type":"vacation","starts_on":"2027-08-02","ends_on":"2027-08-15"}`,
		`{"type":"day_off","starts_on":"2026-12-14","ends_on":"2026-12-14","reason_code":"compensatory_overtime"}`,
		`{"type":"sick","starts_on":"2026-10-01","ends_on":"2026-10-02","certificate_number":"LN-7"}`,
	} {
		do(t, h, http.MethodPost, "/api/requests", raw, map[string]string{"X-Sub": "hr", "X-Name": "Ira", "X-Groups": "hr", "X-Manager": "mgr"}, http.StatusCreated)
	}
	rec = do(t, h, http.MethodGet, "/api/requests", "", map[string]string{"X-Sub": "hr", "X-Name": "Ira", "X-Groups": "hr"}, http.StatusOK)
	if strings.Count(rec, `"employee_id"`) < 3 {
		t.Fatal(rec)
	}
	rec = do(t, h, http.MethodGet, "/api/hr/queue", "", map[string]string{"X-Sub": "hr", "X-Name": "Ira", "X-Groups": "hr"}, http.StatusOK)
	if !strings.Contains(rec, "queue") {
		t.Fatal(rec)
	}
	pdf := doRaw(t, h, http.MethodGet, "/api/hr/export?scope=all&format=pdf", nil, map[string]string{"X-Sub": "hr", "X-Name": "Ira", "X-Groups": "hr"}, http.StatusOK)
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("pdf %q", pdf[:min(8, len(pdf))])
	}
	xls := doRaw(t, h, http.MethodGet, "/api/hr/export?scope=user&user_id="+mustID(t, h, "ada")+"&format=xls", nil, map[string]string{"X-Sub": "hr", "X-Name": "Ira", "X-Groups": "hr"}, http.StatusOK)
	if !bytes.Contains(xls, []byte(">ada<")) || bytes.Contains(xls, []byte(">Ira<")) {
		t.Fatalf("xls scope %s", xls)
	}
	if !strings.Contains(string(api.OpenAPI), "planned_days") || !strings.Contains(string(api.OpenAPI), "/hr/export") || !strings.Contains(string(api.OpenAPI), "/hr/requests/{id}/finance") || !strings.Contains(string(api.OpenAPI), "/intranet") {
		t.Fatal("openapi")
	}
}

func TestIntranetLink(t *testing.T) {
	mem := store.NewMemory()
	svc := portal.New(mem, "hr", config.ClaimNames{
		HireDate: "hire_date", Manager: "manager", Department: "department", EmployeeNumber: "employee_number",
	})
	h := (&Server{Svc: svc, Auth: headerAuth{}, Ready: mem.Ping}).Handler()

	do(t, h, http.MethodGet, "/api/intranet", "", nil, http.StatusUnauthorized)
	do(t, h, http.MethodGet, "/api/me", "", map[string]string{"X-Sub": "mgr", "X-Name": "Mia"}, http.StatusOK)
	do(t, h, http.MethodGet, "/api/me", "", map[string]string{"X-Sub": "ada", "X-Name": "Ada", "X-Manager": "mgr"}, http.StatusOK)

	ada := intranetDoc(t, h, map[string]string{"X-Sub": "ada", "X-Name": "Ada"})
	if ada.User.Subject != "ada" || ada.User.ID == "" || !ada.User.Active || ada.User.Email != "ada@example.com" {
		t.Fatalf("user %+v", ada.User)
	}
	if ids := moduleIDs(ada); ids != "employee" {
		t.Fatalf("ada modules %s", ids)
	}
	if path := linkPath(ada, "employee", "request"); path != "/requests/{id}" {
		t.Fatalf("request link %s", path)
	}
	if linkPath(ada, "employee", "new") != "/requests/new" || linkPath(ada, "employee", "open") != "/" {
		t.Fatalf("employee links %+v", ada.Modules)
	}

	mgr := intranetDoc(t, h, map[string]string{"X-Sub": "mgr", "X-Name": "Mia"})
	if ids := moduleIDs(mgr); ids != "employee,manager" {
		t.Fatalf("mgr modules %s", ids)
	}
	if linkPath(mgr, "manager", "open") != "/team" {
		t.Fatalf("manager link %+v", mgr.Modules)
	}

	hr := intranetDoc(t, h, map[string]string{"X-Sub": "hr", "X-Name": "Ira", "X-Groups": "hr"})
	if ids := moduleIDs(hr); ids != "employee,hr" {
		t.Fatalf("hr modules %s", ids)
	}
	if linkPath(hr, "hr", "open") != "/hr" || linkPath(hr, "hr", "reports") != "/hr/reports" {
		t.Fatalf("hr links %+v", hr.Modules)
	}
}

type intranetBody struct {
	User struct {
		Subject string `json:"subject"`
		ID      string `json:"id"`
		Email   string `json:"email"`
		Active  bool   `json:"active"`
	} `json:"user"`
	Modules []struct {
		ID    string `json:"id"`
		Links []struct {
			Rel  string `json:"rel"`
			Path string `json:"path"`
		} `json:"links"`
	} `json:"modules"`
}

func intranetDoc(t *testing.T, h http.Handler, headers map[string]string) intranetBody {
	t.Helper()
	raw := do(t, h, http.MethodGet, "/api/intranet", "", headers, http.StatusOK)
	var doc intranetBody
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatal(raw)
	}
	return doc
}

func moduleIDs(doc intranetBody) string {
	ids := make([]string, 0, len(doc.Modules))
	for _, m := range doc.Modules {
		ids = append(ids, m.ID)
	}
	return strings.Join(ids, ",")
}

func linkPath(doc intranetBody, module, rel string) string {
	for _, m := range doc.Modules {
		if m.ID != module {
			continue
		}
		for _, link := range m.Links {
			if link.Rel == rel {
				return link.Path
			}
		}
	}
	return ""
}

type sentMail struct {
	to   string
	html string
}

func (s *sentMail) Send(_ context.Context, msg notify.Message) error {
	s.to = msg.To
	s.html = msg.HTML
	return nil
}

func mustID(t *testing.T, h http.Handler, sub string) string {
	t.Helper()
	rec := do(t, h, http.MethodGet, "/api/me", "", map[string]string{"X-Sub": sub, "X-Name": sub}, http.StatusOK)
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(rec), &p); err != nil || p.ID == "" {
		t.Fatal(rec)
	}
	return p.ID
}

func do(t *testing.T, h http.Handler, method, path, body string, headers map[string]string, want int) string {
	t.Helper()
	raw := doRaw(t, h, method, path, []byte(body), headers, want)
	return string(raw)
}

func doRaw(t *testing.T, h http.Handler, method, path string, body []byte, headers map[string]string, want int) []byte {
	t.Helper()
	var rdr *bytes.Reader
	if body == nil {
		rdr = bytes.NewReader(nil)
	} else {
		rdr = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("%s %s -> %d %s", method, path, rec.Code, rec.Body.String())
	}
	return rec.Body.Bytes()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
