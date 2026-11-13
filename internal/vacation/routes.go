// Package vacation is the vacation module: routes, tables, and mail on the shared host.
package vacation

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/wgh-control/bkl-vacation/internal/identity"
	"github.com/wgh-control/bkl-vacation/internal/platform"
	"github.com/wgh-control/bkl-vacation/internal/portal"
)

type handlers struct {
	svc *portal.Service
}

func mount(svc *portal.Service, r platform.Router) {
	h := &handlers{svc: svc}
	r.Authenticated("GET /api/me", h.me)
	r.Authenticated("PATCH /api/me", h.patchMe)
	r.Authenticated("GET /api/calendar", h.calendar)
	r.Authenticated("GET /api/balance", h.balance)
	r.Authenticated("GET /api/me/days", h.days)
	r.Authenticated("GET /api/me/notices", h.notices)
	r.Authenticated("POST /api/me/notices/{id}/read", h.readNotice)
	r.Authenticated("GET /api/day-off-reasons", h.reasons)
	r.Authenticated("POST /api/requests", h.create)
	r.Authenticated("GET /api/requests", h.listMine)
	r.Authenticated("GET /api/requests/{id}", h.getRequest)
	r.Authenticated("POST /api/requests/{id}/cancel", h.cancel)
	r.Authenticated("POST /api/requests/{id}/reschedule", h.reschedule)
	r.Authenticated("POST /api/requests/{id}/confirm", h.confirm)
	r.Authenticated("POST /api/requests/{id}/acknowledge", h.acknowledge)
	r.Authenticated("POST /api/requests/{id}/certificate", h.certificate)
	r.Authenticated("POST /api/requests/{id}/overlap-choice", h.overlap)
	r.Authenticated("GET /api/approvals", h.approvals)
	r.Authenticated("POST /api/approvals/{id}/decision", h.decide)
	r.Authenticated("GET /api/team/requests", h.team)
	r.Authenticated("GET /api/hr/queue", h.hrQueue)
	r.Authenticated("POST /api/hr/requests/{id}/processed", h.processed)
	r.Authenticated("POST /api/hr/requests/{id}/finance", h.finance)
	r.Authenticated("GET /api/hr/export", h.export)
	r.Authenticated("GET /api/hr/warnings", h.warnings)
	r.Authenticated("PUT /api/hr/calendar", h.putCalendar)
	r.Authenticated("POST /api/hr/employees/{id}/entitlement", h.entitlement)
	r.Authenticated("POST /api/hr/schedule/lock", h.lock)
	r.Authenticated("GET /api/hr/schedule", h.schedule)
}

func principal(r *http.Request) identity.Principal {
	p, _ := platform.Principal(r.Context())
	return p
}

func (h *handlers) me(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Me(r.Context(), principal(r))
	respond(w, out, err)
}

func (h *handlers) patchMe(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Locale string `json:"locale"`
	}
	if !decode(w, r, &body) {
		return
	}
	out, err := h.svc.SetLocale(r.Context(), principal(r), body.Locale)
	respond(w, out, err)
}

func (h *handlers) calendar(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Calendar(r.Context(), principal(r), r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	respond(w, map[string]any{"days": out}, err)
}

func (h *handlers) balance(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Balance(r.Context(), principal(r))
	respond(w, out, err)
}

func (h *handlers) days(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.YearDays(r.Context(), principal(r))
	respond(w, out, err)
}

func (h *handlers) notices(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Notices(r.Context(), principal(r))
	respond(w, map[string]any{"notices": out}, err)
}

func (h *handlers) readNotice(w http.ResponseWriter, r *http.Request) {
	err := h.svc.MarkNoticeRead(r.Context(), principal(r), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) reasons(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"reasons": portal.DayOffReasons()})
}

func (h *handlers) create(w http.ResponseWriter, r *http.Request) {
	in, ok := readRequest(w, r)
	if !ok {
		return
	}
	out, err := h.svc.CreateRequest(r.Context(), principal(r), in)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *handlers) listMine(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListMine(r.Context(), principal(r))
	respond(w, map[string]any{"requests": out}, err)
}

func (h *handlers) getRequest(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.GetRequest(r.Context(), principal(r), r.PathValue("id"))
	respond(w, out, err)
}

func (h *handlers) cancel(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Cancel(r.Context(), principal(r), r.PathValue("id"))
	respond(w, out, err)
}

func (h *handlers) reschedule(w http.ResponseWriter, r *http.Request) {
	in, ok := readRequest(w, r)
	if !ok {
		return
	}
	out, err := h.svc.Reschedule(r.Context(), principal(r), r.PathValue("id"), in)
	respond(w, out, err)
}

func (h *handlers) confirm(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Confirm(r.Context(), principal(r), r.PathValue("id"))
	respond(w, out, err)
}

func (h *handlers) acknowledge(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Acknowledge(r.Context(), principal(r), r.PathValue("id"))
	respond(w, out, err)
}

func (h *handlers) certificate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CertificateNumber string `json:"certificate_number"`
	}
	if !decode(w, r, &body) {
		return
	}
	out, err := h.svc.SetCertificate(r.Context(), principal(r), r.PathValue("id"), body.CertificateNumber)
	respond(w, out, err)
}

func (h *handlers) overlap(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Choice string `json:"choice"`
	}
	if !decode(w, r, &body) {
		return
	}
	out, err := h.svc.SetOverlapChoice(r.Context(), principal(r), r.PathValue("id"), body.Choice)
	respond(w, out, err)
}

func (h *handlers) approvals(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListApprovals(r.Context(), principal(r))
	respond(w, map[string]any{"approvals": out}, err)
}

func (h *handlers) decide(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Decision string `json:"decision"`
		Comment  string `json:"comment"`
	}
	if !decode(w, r, &body) {
		return
	}
	out, err := h.svc.Decide(r.Context(), principal(r), r.PathValue("id"), portal.DecisionInput{Decision: body.Decision, Comment: body.Comment})
	respond(w, out, err)
}

func (h *handlers) team(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListTeam(r.Context(), principal(r))
	respond(w, map[string]any{"requests": out}, err)
}

func (h *handlers) hrQueue(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.HRQueue(r.Context(), principal(r))
	respond(w, map[string]any{"queue": out}, err)
}

func (h *handlers) processed(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.MarkProcessed(r.Context(), principal(r), r.PathValue("id"))
	respond(w, out, err)
}

func (h *handlers) finance(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.SendToFinance(r.Context(), principal(r), r.PathValue("id"))
	respond(w, out, err)
}

func (h *handlers) export(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	file, err := h.svc.Export(r.Context(), principal(r), q.Get("scope"), q.Get("user_id"), q.Get("department_id"), q.Get("format"), q.Get("from"), q.Get("to"))
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+file.Name+"\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Body)
}

func (h *handlers) warnings(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Warnings(r.Context(), principal(r))
	respond(w, map[string]any{"warnings": out}, err)
}

func (h *handlers) putCalendar(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date   string `json:"date"`
		Kind   string `json:"kind"`
		Name   string `json:"name"`
		Source string `json:"source"`
	}
	if !decode(w, r, &body) {
		return
	}
	out, err := h.svc.PutCalendarDay(r.Context(), principal(r), body.Date, body.Kind, body.Name, body.Source)
	respond(w, out, err)
}

func (h *handlers) entitlement(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ExtraDays  int  `json:"extra_days"`
		EarlyLeave bool `json:"early_leave"`
	}
	if !decode(w, r, &body) {
		return
	}
	out, err := h.svc.SetEntitlement(r.Context(), principal(r), r.PathValue("id"), body.ExtraDays, body.EarlyLeave)
	respond(w, out, err)
}

func (h *handlers) lock(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Year int `json:"year"`
	}
	if !decode(w, r, &body) {
		return
	}
	out, err := h.svc.LockSchedule(r.Context(), principal(r), body.Year)
	respond(w, out, err)
}

func (h *handlers) schedule(w http.ResponseWriter, r *http.Request) {
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	out, err := h.svc.Schedule(r.Context(), principal(r), year)
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
	platform.WriteJSON(w, status, body)
}
