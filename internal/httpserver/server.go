// Package httpserver is the portal HTTP API. Every path under /api is in the OpenAPI spec.
package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/wgh-control/bkl-vacation/api"
	"github.com/wgh-control/bkl-vacation/internal/identity"
	"github.com/wgh-control/bkl-vacation/internal/portal"
)

type ctxKey struct{}

// Server serves the API.
type Server struct {
	Svc   *portal.Service
	Auth  identity.Authenticator
	Ready func(context.Context) error
	CORS  string
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/healthz", s.health)
	mux.HandleFunc("GET /api/readyz", s.ready)
	mux.HandleFunc("GET /api/openapi.yaml", s.openapi)
	mux.HandleFunc("GET /api/me", s.authed(s.me))
	mux.HandleFunc("PATCH /api/me", s.authed(s.patchMe))
	mux.HandleFunc("GET /api/calendar", s.authed(s.calendar))
	mux.HandleFunc("GET /api/balance", s.authed(s.balance))
	mux.HandleFunc("GET /api/me/days", s.authed(s.days))
	mux.HandleFunc("GET /api/me/notices", s.authed(s.notices))
	mux.HandleFunc("POST /api/me/notices/{id}/read", s.authed(s.readNotice))
	mux.HandleFunc("GET /api/day-off-reasons", s.authed(s.reasons))
	mux.HandleFunc("POST /api/requests", s.authed(s.create))
	mux.HandleFunc("GET /api/requests", s.authed(s.listMine))
	mux.HandleFunc("GET /api/requests/{id}", s.authed(s.getRequest))
	mux.HandleFunc("POST /api/requests/{id}/cancel", s.authed(s.cancel))
	mux.HandleFunc("POST /api/requests/{id}/reschedule", s.authed(s.reschedule))
	mux.HandleFunc("POST /api/requests/{id}/confirm", s.authed(s.confirm))
	mux.HandleFunc("POST /api/requests/{id}/acknowledge", s.authed(s.acknowledge))
	mux.HandleFunc("POST /api/requests/{id}/certificate", s.authed(s.certificate))
	mux.HandleFunc("POST /api/requests/{id}/overlap-choice", s.authed(s.overlap))
	mux.HandleFunc("GET /api/approvals", s.authed(s.approvals))
	mux.HandleFunc("POST /api/approvals/{id}/decision", s.authed(s.decide))
	mux.HandleFunc("GET /api/team/requests", s.authed(s.team))
	mux.HandleFunc("GET /api/hr/queue", s.authed(s.hrQueue))
	mux.HandleFunc("POST /api/hr/requests/{id}/processed", s.authed(s.processed))
	mux.HandleFunc("POST /api/hr/requests/{id}/finance", s.authed(s.finance))
	mux.HandleFunc("GET /api/hr/export", s.authed(s.export))
	mux.HandleFunc("GET /api/hr/warnings", s.authed(s.warnings))
	mux.HandleFunc("PUT /api/hr/calendar", s.authed(s.putCalendar))
	mux.HandleFunc("POST /api/hr/employees/{id}/entitlement", s.authed(s.entitlement))
	mux.HandleFunc("POST /api/hr/schedule/lock", s.authed(s.lock))
	mux.HandleFunc("GET /api/hr/schedule", s.authed(s.schedule))
	return s.cors(mux)
}

func (s *Server) cors(next http.Handler) http.Handler {
	if s.CORS == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.CORS)
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authed(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.Auth == nil {
			writeErr(w, portalErr("unauthorized", "authentication is not configured", 401))
			return
		}
		p, err := s.Auth.Authenticate(r)
		if err != nil || p.Subject == "" {
			writeErr(w, portalErr("unauthorized", "bearer token was rejected", 401))
			return
		}
		fn(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, p)))
	}
}

func principal(r *http.Request) identity.Principal {
	p, _ := r.Context().Value(ctxKey{}).(identity.Principal)
	return p
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if s.Ready != nil {
		if err := s.Ready(r.Context()); err != nil {
			writeErr(w, portalErr("unavailable", "database is not reachable", http.StatusServiceUnavailable))
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) openapi(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	_, _ = w.Write(api.OpenAPI)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	out, err := s.Svc.Me(r.Context(), principal(r))
	respond(w, out, err)
}

func (s *Server) patchMe(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Locale string `json:"locale"`
	}
	if !decode(w, r, &body) {
		return
	}
	out, err := s.Svc.SetLocale(r.Context(), principal(r), body.Locale)
	respond(w, out, err)
}

func (s *Server) calendar(w http.ResponseWriter, r *http.Request) {
	out, err := s.Svc.Calendar(r.Context(), principal(r), r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	respond(w, map[string]any{"days": out}, err)
}

func (s *Server) balance(w http.ResponseWriter, r *http.Request) {
	out, err := s.Svc.Balance(r.Context(), principal(r))
	respond(w, out, err)
}

func (s *Server) days(w http.ResponseWriter, r *http.Request) {
	out, err := s.Svc.YearDays(r.Context(), principal(r))
	respond(w, out, err)
}

func (s *Server) notices(w http.ResponseWriter, r *http.Request) {
	out, err := s.Svc.Notices(r.Context(), principal(r))
	respond(w, map[string]any{"notices": out}, err)
}

func (s *Server) readNotice(w http.ResponseWriter, r *http.Request) {
	err := s.Svc.MarkNoticeRead(r.Context(), principal(r), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) reasons(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"reasons": portal.DayOffReasons()})
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	in, ok := readRequest(w, r)
	if !ok {
		return
	}
	out, err := s.Svc.CreateRequest(r.Context(), principal(r), in)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) listMine(w http.ResponseWriter, r *http.Request) {
	out, err := s.Svc.ListMine(r.Context(), principal(r))
	respond(w, map[string]any{"requests": out}, err)
}

func (s *Server) getRequest(w http.ResponseWriter, r *http.Request) {
	out, err := s.Svc.GetRequest(r.Context(), principal(r), r.PathValue("id"))
	respond(w, out, err)
}

func (s *Server) cancel(w http.ResponseWriter, r *http.Request) {
	out, err := s.Svc.Cancel(r.Context(), principal(r), r.PathValue("id"))
	respond(w, out, err)
}

func (s *Server) reschedule(w http.ResponseWriter, r *http.Request) {
	in, ok := readRequest(w, r)
	if !ok {
		return
	}
	out, err := s.Svc.Reschedule(r.Context(), principal(r), r.PathValue("id"), in)
	respond(w, out, err)
}

func (s *Server) confirm(w http.ResponseWriter, r *http.Request) {
	out, err := s.Svc.Confirm(r.Context(), principal(r), r.PathValue("id"))
	respond(w, out, err)
}

func (s *Server) acknowledge(w http.ResponseWriter, r *http.Request) {
	out, err := s.Svc.Acknowledge(r.Context(), principal(r), r.PathValue("id"))
	respond(w, out, err)
}

func (s *Server) certificate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CertificateNumber string `json:"certificate_number"`
	}
	if !decode(w, r, &body) {
		return
	}
	out, err := s.Svc.SetCertificate(r.Context(), principal(r), r.PathValue("id"), body.CertificateNumber)
	respond(w, out, err)
}

func (s *Server) overlap(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Choice string `json:"choice"`
	}
	if !decode(w, r, &body) {
		return
	}
	out, err := s.Svc.SetOverlapChoice(r.Context(), principal(r), r.PathValue("id"), body.Choice)
	respond(w, out, err)
}

func (s *Server) approvals(w http.ResponseWriter, r *http.Request) {
	out, err := s.Svc.ListApprovals(r.Context(), principal(r))
	respond(w, map[string]any{"approvals": out}, err)
}

func (s *Server) decide(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Decision string `json:"decision"`
		Comment  string `json:"comment"`
	}
	if !decode(w, r, &body) {
		return
	}
	out, err := s.Svc.Decide(r.Context(), principal(r), r.PathValue("id"), portal.DecisionInput{Decision: body.Decision, Comment: body.Comment})
	respond(w, out, err)
}

func (s *Server) team(w http.ResponseWriter, r *http.Request) {
	out, err := s.Svc.ListTeam(r.Context(), principal(r))
	respond(w, map[string]any{"requests": out}, err)
}

func (s *Server) hrQueue(w http.ResponseWriter, r *http.Request) {
	out, err := s.Svc.HRQueue(r.Context(), principal(r))
	respond(w, map[string]any{"queue": out}, err)
}

func (s *Server) processed(w http.ResponseWriter, r *http.Request) {
	out, err := s.Svc.MarkProcessed(r.Context(), principal(r), r.PathValue("id"))
	respond(w, out, err)
}

func (s *Server) finance(w http.ResponseWriter, r *http.Request) {
	out, err := s.Svc.SendToFinance(r.Context(), principal(r), r.PathValue("id"))
	respond(w, out, err)
}

func (s *Server) export(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	file, err := s.Svc.Export(r.Context(), principal(r), q.Get("scope"), q.Get("user_id"), q.Get("department_id"), q.Get("format"), q.Get("from"), q.Get("to"))
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+file.Name+"\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Body)
}

func (s *Server) warnings(w http.ResponseWriter, r *http.Request) {
	out, err := s.Svc.Warnings(r.Context(), principal(r))
	respond(w, map[string]any{"warnings": out}, err)
}

func (s *Server) putCalendar(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date   string `json:"date"`
		Kind   string `json:"kind"`
		Name   string `json:"name"`
		Source string `json:"source"`
	}
	if !decode(w, r, &body) {
		return
	}
	out, err := s.Svc.PutCalendarDay(r.Context(), principal(r), body.Date, body.Kind, body.Name, body.Source)
	respond(w, out, err)
}

func (s *Server) entitlement(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ExtraDays  int  `json:"extra_days"`
		EarlyLeave bool `json:"early_leave"`
	}
	if !decode(w, r, &body) {
		return
	}
	out, err := s.Svc.SetEntitlement(r.Context(), principal(r), r.PathValue("id"), body.ExtraDays, body.EarlyLeave)
	respond(w, out, err)
}

func (s *Server) lock(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Year int `json:"year"`
	}
	if !decode(w, r, &body) {
		return
	}
	out, err := s.Svc.LockSchedule(r.Context(), principal(r), body.Year)
	respond(w, out, err)
}

func (s *Server) schedule(w http.ResponseWriter, r *http.Request) {
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	out, err := s.Svc.Schedule(r.Context(), principal(r), year)
	respond(w, out, err)
}

func readRequest(w http.ResponseWriter, r *http.Request) (portal.CreateInput, bool) {
	var body struct {
		Type              string `json:"type"`
		StartsOn          string `json:"starts_on"`
		EndsOn            string `json:"ends_on"`
		CountedDays       int    `json:"counted_days"`
		ReasonCode        string `json:"reason_code"`
		CertificateNumber string `json:"certificate_number"`
		OverlapChoice     string `json:"overlap_choice"`
	}
	if !decode(w, r, &body) {
		return portal.CreateInput{}, false
	}
	return portal.CreateInput{
		Type: body.Type, StartsOn: body.StartsOn, EndsOn: body.EndsOn, CountedDays: body.CountedDays,
		ReasonCode: body.ReasonCode, CertificateNumber: body.CertificateNumber, OverlapChoice: body.OverlapChoice,
	}, true
}

func decode(w http.ResponseWriter, r *http.Request, dest any) bool {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		writeErr(w, portalErr("validation", "request body is not valid JSON", 400))
		return false
	}
	return true
}

func respond(w http.ResponseWriter, body any, err error) {
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func writeErr(w http.ResponseWriter, err error) {
	var pe *portal.Error
	if errors.As(err, &pe) {
		writeJSON(w, pe.Status, map[string]string{"code": pe.Code, "message": pe.Message})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "internal", "message": "internal error"})
}

func portalErr(code, msg string, status int) error {
	return &portal.Error{Code: code, Message: msg, Status: status}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
