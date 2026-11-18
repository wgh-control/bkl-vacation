package platform

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wgh-control/bkl-vacation/internal/identity"
	"github.com/wgh-control/bkl-vacation/internal/notify"
)

type stubAuth struct{}

func (stubAuth) Authenticate(r *http.Request) (identity.Principal, error) {
	sub := r.Header.Get("X-Sub")
	if sub == "" {
		return identity.Principal{}, errNoSub{}
	}
	return identity.Principal{Subject: sub, Name: sub}, nil
}

type errNoSub struct{}

func (errNoSub) Error() string { return "missing" }

type eduModule struct {
	dispatched int
	senderNil  bool
}

func (eduModule) Name() string { return "education" }

func (eduModule) Tables() []Table {
	return []Table{{
		Version: "education/001_requests.sql",
		SQL:     "CREATE TABLE education_request (id UUID PRIMARY KEY);",
	}}
}

func (eduModule) Routes(r Router) {
	r.Handle("GET /api/education/health", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"module": "education"})
	})
	r.Authenticated("POST /api/education/requests", func(w http.ResponseWriter, r *http.Request) {
		p, ok := Principal(r.Context())
		if !ok || p.Subject == "" {
			WriteJSON(w, http.StatusInternalServerError, map[string]string{"code": "internal", "message": "principal missing"})
			return
		}
		WriteJSON(w, http.StatusCreated, map[string]string{"module": "education", "subject": p.Subject})
	})
}

func (e *eduModule) Mail() Mailer { return e }

func (e *eduModule) Dispatch(_ context.Context, sender notify.Sender) error {
	e.dispatched++
	e.senderNil = sender == nil
	return nil
}

type otherModule struct{ dispatched int }

func (otherModule) Name() string { return "other" }
func (otherModule) Tables() []Table {
	return nil
}
func (otherModule) Routes(r Router) {
	r.Authenticated("GET /api/other", func(w http.ResponseWriter, r *http.Request) {
		p, _ := Principal(r.Context())
		WriteJSON(w, http.StatusOK, map[string]string{"subject": p.Subject})
	})
}
func (o *otherModule) Mail() Mailer { return o }
func (o *otherModule) Dispatch(context.Context, notify.Sender) error {
	o.dispatched++
	return nil
}

func TestModulesShareOneAPI(t *testing.T) {
	edu := &eduModule{}
	other := &otherModule{}
	h := Handler(Deps{
		Auth:    stubAuth{},
		Ready:   func(context.Context) error { return nil },
		OpenAPI: []byte("openapi: 3.0.3\n"),
		Modules: []Module{other, edu},
	})

	if rec := call(t, h, http.MethodGet, "/api/healthz", nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatal(rec.Body.String())
	}
	if rec := call(t, h, http.MethodGet, "/api/readyz", nil); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	if rec := call(t, h, http.MethodGet, "/api/openapi.yaml", nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "openapi: 3.0.3") {
		t.Fatal(rec.Body.String())
	}
	if rec := call(t, h, http.MethodGet, "/api/education/health", nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"module":"education"`) {
		t.Fatal(rec.Body.String())
	}
	if rec := call(t, h, http.MethodPost, "/api/education/requests", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated education route: %d %s", rec.Code, rec.Body.String())
	}
	rec := call(t, h, http.MethodPost, "/api/education/requests", map[string]string{"X-Sub": "ada"})
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"subject":"ada"`) {
		t.Fatal(rec.Body.String())
	}
	rec = call(t, h, http.MethodGet, "/api/other", map[string]string{"X-Sub": "ada"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"subject":"ada"`) {
		t.Fatal(rec.Body.String())
	}

	if err := DispatchAll(context.Background(), []Module{other, edu}, nil); err != nil {
		t.Fatal(err)
	}
	if edu.dispatched != 1 || !edu.senderNil || other.dispatched != 1 {
		t.Fatalf("mail edu=%d other=%d nil=%v", edu.dispatched, other.dispatched, edu.senderNil)
	}
}

func TestRejectsDuplicateMigrationVersions(t *testing.T) {
	err := Apply(context.Background(), nil, []Table{
		{Version: "education/001_requests.sql", SQL: "select 1"},
		{Version: "education/001_requests.sql", SQL: "select 1"},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate migration version") {
		t.Fatal(err)
	}
	err = Apply(context.Background(), nil, []Table{{Version: "  ", SQL: "select 1"}})
	if err == nil || !strings.Contains(err.Error(), "version is empty") {
		t.Fatal(err)
	}
}

func call(t *testing.T, h http.Handler, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, io.NopCloser(strings.NewReader("")))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
