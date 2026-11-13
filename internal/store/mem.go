package store

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/wgh-control/bkl-vacation/internal/model"
)

// Memory is a process-local repository. A transaction holds the mutex for its whole body.
type Memory struct {
	mu       sync.Mutex
	advisory bool

	employees map[string]model.Employee
	bySubject map[string]string
	units     map[string]model.OrgUnit
	years     map[string]model.LeaveYear
	requests  map[string]model.Request
	steps     map[string]model.Step
	days      map[string]model.CalendarDay
	notes     map[string]model.Notification
	notices   map[string]model.Notice
	audits    []model.Audit
	locks     map[int]model.ScheduleLock
}

// NewMemory returns an empty repository.
func NewMemory() *Memory {
	return &Memory{
		employees: map[string]model.Employee{},
		bySubject: map[string]string{},
		units:     map[string]model.OrgUnit{},
		years:     map[string]model.LeaveYear{},
		requests:  map[string]model.Request{},
		steps:     map[string]model.Step{},
		days:      map[string]model.CalendarDay{},
		notes:     map[string]model.Notification{},
		notices:   map[string]model.Notice{},
		locks:     map[int]model.ScheduleLock{},
	}
}

func (m *Memory) Ping(context.Context) error { return nil }

func (m *Memory) View(ctx context.Context, fn func(Queries) error) error {
	return m.RunInTx(ctx, fn)
}

func (m *Memory) RunInTx(_ context.Context, fn func(Queries) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fn(m)
}

func (m *Memory) WithAdvisoryLock(ctx context.Context, fn func(context.Context) error) (bool, error) {
	m.mu.Lock()
	if m.advisory {
		m.mu.Unlock()
		return false, nil
	}
	m.advisory = true
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.advisory = false
		m.mu.Unlock()
	}()
	return true, fn(ctx)
}

func (m *Memory) UpsertEmployee(_ context.Context, e model.Employee) (model.Employee, error) {
	if id, ok := m.bySubject[e.Subject]; ok {
		cur := m.employees[id]
		e.ID = cur.ID
		e.CreatedAt = cur.CreatedAt
		m.employees[id] = e
		return e, nil
	}
	if e.ID == "" {
		return model.Employee{}, ErrNotFound
	}
	m.employees[e.ID] = e
	m.bySubject[e.Subject] = e.ID
	return e, nil
}

func (m *Memory) UpdateEmployee(_ context.Context, e model.Employee) error {
	cur, ok := m.employees[e.ID]
	if !ok {
		return ErrNotFound
	}
	delete(m.bySubject, cur.Subject)
	m.employees[e.ID] = e
	m.bySubject[e.Subject] = e.ID
	return nil
}

func (m *Memory) GetEmployee(_ context.Context, id string) (model.Employee, error) {
	e, ok := m.employees[id]
	if !ok {
		return model.Employee{}, ErrNotFound
	}
	return e, nil
}

func (m *Memory) GetEmployeeBySubject(_ context.Context, subject string) (model.Employee, error) {
	id, ok := m.bySubject[subject]
	if !ok {
		return model.Employee{}, ErrNotFound
	}
	return m.employees[id], nil
}

func (m *Memory) ListEmployees(context.Context) ([]model.Employee, error) {
	out := make([]model.Employee, 0, len(m.employees))
	for _, e := range m.employees {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DisplayName < out[j].DisplayName })
	return out, nil
}

func (m *Memory) UpsertOrgUnit(_ context.Context, u model.OrgUnit) (model.OrgUnit, error) {
	for id, cur := range m.units {
		if cur.ExternalID == u.ExternalID {
			u.ID = id
			u.CreatedAt = cur.CreatedAt
			m.units[id] = u
			return u, nil
		}
	}
	m.units[u.ID] = u
	return u, nil
}

func (m *Memory) GetOrgUnit(_ context.Context, id string) (model.OrgUnit, error) {
	u, ok := m.units[id]
	if !ok {
		return model.OrgUnit{}, ErrNotFound
	}
	return u, nil
}

func (m *Memory) ListOrgUnits(context.Context) ([]model.OrgUnit, error) {
	out := make([]model.OrgUnit, 0, len(m.units))
	for _, u := range m.units {
		out = append(out, u)
	}
	return out, nil
}

func (m *Memory) UpsertLeaveYear(_ context.Context, y model.LeaveYear) (model.LeaveYear, error) {
	for id, cur := range m.years {
		if cur.EmployeeID == y.EmployeeID && sameDay(cur.StartsOn, y.StartsOn) {
			y.ID = id
			y.CreatedAt = cur.CreatedAt
			m.years[id] = y
			return y, nil
		}
	}
	m.years[y.ID] = y
	return y, nil
}

func (m *Memory) ListLeaveYears(_ context.Context, employeeID string) ([]model.LeaveYear, error) {
	var out []model.LeaveYear
	for _, y := range m.years {
		if y.EmployeeID == employeeID {
			out = append(out, y)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartsOn.Before(out[j].StartsOn) })
	return out, nil
}

func (m *Memory) LockLeaveYear(_ context.Context, id string) error {
	if _, ok := m.years[id]; !ok {
		return ErrNotFound
	}
	return nil
}

func (m *Memory) InsertRequest(_ context.Context, r model.Request) error {
	if _, ok := m.requests[r.ID]; ok {
		return ErrNotFound
	}
	m.requests[r.ID] = r
	return nil
}

func (m *Memory) UpdateRequest(_ context.Context, r model.Request) error {
	if _, ok := m.requests[r.ID]; !ok {
		return ErrNotFound
	}
	m.requests[r.ID] = r
	return nil
}

func (m *Memory) GetRequest(_ context.Context, id string) (model.Request, error) {
	r, ok := m.requests[id]
	if !ok {
		return model.Request{}, ErrNotFound
	}
	return r, nil
}

func (m *Memory) ListRequests(_ context.Context, f model.RequestFilter) ([]model.Request, error) {
	var out []model.Request
	for _, r := range m.requests {
		emp := m.employees[r.EmployeeID]
		if !matchRequest(f, r, emp) {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartsOn.Equal(out[j].StartsOn) {
			return out[i].ID < out[j].ID
		}
		return out[i].StartsOn.Before(out[j].StartsOn)
	})
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func matchRequest(f model.RequestFilter, r model.Request, emp model.Employee) bool {
	if f.EmployeeID != "" && r.EmployeeID != f.EmployeeID {
		return false
	}
	if f.ManagerID != "" && emp.ManagerID != f.ManagerID {
		return false
	}
	if f.DepartmentID != "" && emp.DepartmentID != f.DepartmentID {
		return false
	}
	if len(f.Statuses) > 0 && !contains(f.Statuses, string(r.Status)) {
		return false
	}
	if f.EndsOnOrAfter != nil && day(r.EndsOn).Before(day(*f.EndsOnOrAfter)) {
		return false
	}
	if f.OverlapFrom != nil && f.OverlapTo != nil {
		if day(r.EndsOn).Before(day(*f.OverlapFrom)) || day(r.StartsOn).After(day(*f.OverlapTo)) {
			return false
		}
	}
	return true
}

func (m *Memory) InsertStep(_ context.Context, s model.Step) error {
	m.steps[s.ID] = s
	return nil
}

func (m *Memory) UpdateStep(_ context.Context, s model.Step) error {
	if _, ok := m.steps[s.ID]; !ok {
		return ErrNotFound
	}
	m.steps[s.ID] = s
	return nil
}

func (m *Memory) ListSteps(_ context.Context, requestID string) ([]model.Step, error) {
	var out []model.Step
	for _, s := range m.steps {
		if s.RequestID == requestID {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Sequence < out[j].Sequence })
	return out, nil
}

func (m *Memory) UpsertCalendarDay(_ context.Context, d model.CalendarDay) error {
	m.days[dayKey(d.Date)] = d
	return nil
}

func (m *Memory) ListCalendarDays(_ context.Context, from, to time.Time) ([]model.CalendarDay, error) {
	var out []model.CalendarDay
	for _, d := range m.days {
		if !day(d.Date).Before(day(from)) && !day(d.Date).After(day(to)) {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out, nil
}

func (m *Memory) InsertNotification(_ context.Context, n model.Notification) error {
	for _, cur := range m.notes {
		if cur.RequestID == n.RequestID && cur.Kind == n.Kind && cur.RecipientEmployeeID == n.RecipientEmployeeID {
			return nil
		}
	}
	m.notes[n.ID] = n
	return nil
}

func (m *Memory) ListUnsentDue(_ context.Context, now time.Time) ([]model.Notification, error) {
	var out []model.Notification
	for _, n := range m.notes {
		if n.SentAt == nil && !n.DueAt.After(now) {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DueAt.Before(out[j].DueAt) })
	return out, nil
}

func (m *Memory) UpdateNotification(_ context.Context, n model.Notification) error {
	if _, ok := m.notes[n.ID]; !ok {
		return ErrNotFound
	}
	m.notes[n.ID] = n
	return nil
}

func (m *Memory) InsertNotice(_ context.Context, n model.Notice) error {
	m.notices[n.ID] = n
	return nil
}

func (m *Memory) ListNotices(_ context.Context, employeeID string) ([]model.Notice, error) {
	var out []model.Notice
	for _, n := range m.notices {
		if n.EmployeeID == employeeID {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (m *Memory) MarkNoticeRead(_ context.Context, id, employeeID string, at time.Time) error {
	n, ok := m.notices[id]
	if !ok || n.EmployeeID != employeeID {
		return ErrNotFound
	}
	n.ReadAt = &at
	m.notices[id] = n
	return nil
}

func (m *Memory) InsertAudit(_ context.Context, a model.Audit) error {
	m.audits = append(m.audits, a)
	return nil
}

func (m *Memory) UpsertScheduleLock(_ context.Context, lock model.ScheduleLock) error {
	m.locks[lock.Year] = lock
	return nil
}

func (m *Memory) GetScheduleLock(_ context.Context, year int) (model.ScheduleLock, error) {
	lock, ok := m.locks[year]
	if !ok {
		return model.ScheduleLock{}, ErrNotFound
	}
	return lock, nil
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func day(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func sameDay(a, b time.Time) bool { return day(a).Equal(day(b)) }

func dayKey(t time.Time) string { return day(t).Format("2006-01-02") }
