package portal

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/wgh-control/bkl-vacation/internal/config"
	"github.com/wgh-control/bkl-vacation/internal/identity"
	"github.com/wgh-control/bkl-vacation/internal/leave"
	"github.com/wgh-control/bkl-vacation/internal/model"
	"github.com/wgh-control/bkl-vacation/internal/notify"
	"github.com/wgh-control/bkl-vacation/internal/org"
	"github.com/wgh-control/bkl-vacation/internal/store"
)

// Service is the vacation portal. Both the API and the worker call it.
type Service struct {
	Repo         store.Repo
	Now          func() time.Time
	NewID        func() string
	HRGroup      string
	Claims       config.ClaimNames
	Mail         notify.Sender
	FinanceEmail string
}

func New(repo store.Repo, hrGroup string, claims config.ClaimNames) *Service {
	if hrGroup == "" {
		hrGroup = "hr"
	}
	return &Service{
		Repo:    repo,
		Now:     func() time.Time { return time.Now().UTC() },
		NewID:   func() string { return uuid.NewString() },
		HRGroup: hrGroup,
		Claims:  claims,
	}
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) id() string {
	if s.NewID != nil {
		return s.NewID()
	}
	return uuid.NewString()
}

func (s *Service) tx(ctx context.Context, fn func(store.Queries) error) error {
	return s.Repo.RunInTx(ctx, fn)
}

// Me returns the signed-in person, creating the portal row from the token on first login.
func (s *Service) Me(ctx context.Context, p identity.Principal) (Profile, error) {
	var out Profile
	err := s.tx(ctx, func(q store.Queries) error {
		emp, err := s.upsertFromPrincipal(ctx, q, p)
		if err != nil {
			return err
		}
		out, err = s.profile(ctx, q, emp)
		return err
	})
	return out, err
}

// SetLocale stores the UI language. Mail uses the same preference. Russian is the default.
func (s *Service) SetLocale(ctx context.Context, p identity.Principal, locale string) (Profile, error) {
	if locale != "ru" && locale != "en" {
		return Profile{}, code("locale", "locale must be ru or en", 400)
	}
	var out Profile
	err := s.tx(ctx, func(q store.Queries) error {
		emp, err := s.upsertFromPrincipal(ctx, q, p)
		if err != nil {
			return err
		}
		if !emp.Active {
			return code("inactive", "inactive accounts cannot change settings", 403)
		}
		emp.Locale = locale
		emp.LocaleChosen = true
		emp.UpdatedAt = s.now()
		if err := q.UpdateEmployee(ctx, emp); err != nil {
			return err
		}
		out, err = s.profile(ctx, q, emp)
		return err
	})
	return out, err
}

func (s *Service) upsertFromPrincipal(ctx context.Context, q store.Queries, p identity.Principal) (model.Employee, error) {
	if strings.TrimSpace(p.Subject) == "" {
		return model.Employee{}, code("unauthorized", "missing subject", 401)
	}
	now := s.now()
	existing, err := q.GetEmployeeBySubject(ctx, p.Subject)
	found := err == nil
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return model.Employee{}, err
	}
	emp := existing
	if !found {
		emp = model.Employee{
			ID: s.id(), Subject: p.Subject, Locale: "ru", Active: true,
			CreatedAt: now, Raw: []byte(`{}`),
		}
	}
	if p.Name != "" {
		emp.DisplayName = p.Name
	}
	if emp.DisplayName == "" {
		emp.DisplayName = p.Subject
	}
	if email := first(p.Email, p.Claim(s.Claims.Email)); email != "" {
		emp.Email = email
	}
	if !emp.LocaleChosen {
		if loc := p.Claim(s.Claims.Locale); loc == "ru" || loc == "en" {
			emp.Locale = loc
		}
		if emp.Locale == "" {
			emp.Locale = "ru"
		}
	}
	if v := p.Claim(s.Claims.EmployeeNumber); v != "" {
		emp.EmployeeNumber = v
	}
	if v := p.Claim(s.Claims.Position); v != "" {
		emp.Position = v
	}
	if v := p.Claim(s.Claims.HireDate); v != "" {
		if t, err := leave.ParseDate(v); err == nil {
			emp.HireDate = &t
		}
	}
	emp.HR = p.HasGroup(s.HRGroup)
	emp.Active = true
	emp.UpdatedAt = now
	if dept := p.Claim(s.Claims.Department); dept != "" {
		unit, err := s.ensureDepartment(ctx, q, dept, now)
		if err != nil {
			return model.Employee{}, err
		}
		emp.DepartmentID = unit.ID
	}
	managerRaw := p.Claim(s.Claims.Manager)
	if managerRaw != "" {
		emp.ManagerRaw = managerRaw
	}
	if !found {
		saved, err := q.UpsertEmployee(ctx, emp)
		if err != nil {
			return model.Employee{}, err
		}
		emp = saved
	}
	if managerRaw != "" {
		if mgr, err := s.resolveManager(ctx, q, managerRaw); err == nil && mgr.ID != emp.ID {
			emp.ManagerID = mgr.ID
		}
	}
	if err := q.UpdateEmployee(ctx, emp); err != nil {
		return model.Employee{}, err
	}
	return emp, nil
}

func (s *Service) resolveManager(ctx context.Context, q store.Queries, raw string) (model.Employee, error) {
	if e, err := q.GetEmployeeBySubject(ctx, raw); err == nil {
		return e, nil
	}
	all, err := q.ListEmployees(ctx)
	if err != nil {
		return model.Employee{}, err
	}
	for _, e := range all {
		if e.ID == raw || (e.Email != "" && strings.EqualFold(e.Email, raw)) || (e.EmployeeNumber != "" && e.EmployeeNumber == raw) {
			return e, nil
		}
	}
	return model.Employee{}, store.ErrNotFound
}

func (s *Service) ensureDepartment(ctx context.Context, q store.Queries, path string, now time.Time) (model.OrgUnit, error) {
	external, name, parent := org.DepartmentPath(path)
	if external == "" {
		return model.OrgUnit{}, code("validation", "empty department", 400)
	}
	var parentID string
	if parent != "" {
		p, err := s.ensureDepartment(ctx, q, parent, now)
		if err != nil {
			return model.OrgUnit{}, err
		}
		parentID = p.ID
	}
	units, err := q.ListOrgUnits(ctx)
	if err != nil {
		return model.OrgUnit{}, err
	}
	for _, u := range units {
		if u.ExternalID == external {
			if u.Name != name || u.ParentID != parentID {
				u.Name = name
				u.ParentID = parentID
				u.UpdatedAt = now
				return q.UpsertOrgUnit(ctx, u)
			}
			return u, nil
		}
	}
	return q.UpsertOrgUnit(ctx, model.OrgUnit{
		ID: s.id(), ExternalID: external, Name: name, ParentID: parentID, CreatedAt: now, UpdatedAt: now,
	})
}

func (s *Service) profile(ctx context.Context, q store.Queries, emp model.Employee) (Profile, error) {
	out := Profile{
		ID: emp.ID, Subject: emp.Subject, DisplayName: emp.DisplayName, Email: emp.Email,
		Locale: emp.Locale, EmployeeNumber: emp.EmployeeNumber, Position: emp.Position,
		Active: emp.Active, HR: emp.HR, ExtraDays: emp.ExtraDays, EarlyLeave: emp.EarlyLeave,
	}
	if emp.Locale == "" {
		out.Locale = "ru"
	}
	if emp.HireDate != nil {
		out.HireDate = leave.FormatDate(*emp.HireDate)
	}
	if emp.DepartmentID != "" {
		if u, err := q.GetOrgUnit(ctx, emp.DepartmentID); err == nil {
			out.Department = &Ref{ID: u.ID, Name: u.Name}
		}
	}
	if emp.ManagerID != "" {
		if m, err := q.GetEmployee(ctx, emp.ManagerID); err == nil {
			out.Manager = &Ref{ID: m.ID, Name: m.DisplayName}
		}
	}
	all, err := q.ListEmployees(ctx)
	if err != nil {
		return Profile{}, err
	}
	for _, other := range all {
		if other.ManagerID == emp.ID && other.ID != emp.ID {
			out.ManagerOfOthers = true
			break
		}
	}
	return out, nil
}

func (s *Service) require(ctx context.Context, q store.Queries, p identity.Principal, write bool) (model.Employee, error) {
	emp, err := s.upsertFromPrincipal(ctx, q, p)
	if err != nil {
		return model.Employee{}, err
	}
	if write && !emp.Active {
		return model.Employee{}, code("inactive", "inactive accounts cannot submit requests", 403)
	}
	return emp, nil
}

func (s *Service) requireHR(ctx context.Context, q store.Queries, p identity.Principal) (model.Employee, error) {
	emp, err := s.require(ctx, q, p, true)
	if err != nil {
		return model.Employee{}, err
	}
	if !p.HasGroup(s.HRGroup) {
		return model.Employee{}, code("forbidden", "HR group required", 403)
	}
	return emp, nil
}

func first(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func stamp(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func checkComment(s string) error {
	if utf8.RuneCountInString(s) > 2000 {
		return code("validation", "comment is too long", 400)
	}
	return nil
}
