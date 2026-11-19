package portal

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/wgh-control/bkl-vacation/internal/identity"
	"github.com/wgh-control/bkl-vacation/internal/model"
	"github.com/wgh-control/bkl-vacation/internal/notify"
	"github.com/wgh-control/bkl-vacation/internal/store"
)

// applyADManager stores the manager from the directory.
// Keycloak already exposes the Active Directory manager attribute, which is
// the Exchange reporting hierarchy. This does not open another directory.
// A manual override stays in place. When the directory manager changes under
// an override, the new directory value is recorded and HR is mailed.
func (s *Service) applyADManager(ctx context.Context, q store.Queries, emp *model.Employee, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" || emp == nil || emp.ID == "" {
		return nil
	}
	resolved := ""
	if mgr, err := s.resolveManager(ctx, q, raw); err == nil && mgr.ID != "" && mgr.ID != emp.ID {
		resolved = mgr.ID
	}
	if resolved != "" && resolved == emp.ADManagerID {
		if emp.ManagerRaw == raw && (emp.ManagerOverrideID == "" || emp.ManagerID == emp.ManagerOverrideID) {
			return nil
		}
		emp.ManagerRaw = raw
		if emp.ManagerOverrideID != "" {
			emp.ManagerID = emp.ManagerOverrideID
		}
		emp.UpdatedAt = s.now()
		return q.UpdateEmployee(ctx, *emp)
	}
	if resolved == "" && raw == emp.ManagerRaw {
		return nil
	}
	prevID := emp.ADManagerID
	emp.ADManagerID = resolved
	emp.ManagerRaw = raw
	emp.UpdatedAt = s.now()
	if emp.ManagerOverrideID == "" {
		emp.ManagerID = resolved
		emp.ManagerConflictADID = ""
		emp.ManagerConflictRaw = ""
		if err := q.UpdateEmployee(ctx, *emp); err != nil {
			return err
		}
		if prevID != "" && prevID != resolved {
			return s.audit(ctx, q, "", "", emp.ID, "manager_ad_changed", prevID, resolved)
		}
		return nil
	}
	emp.ManagerID = emp.ManagerOverrideID
	if resolved != "" && resolved == emp.ManagerOverrideID {
		emp.ManagerConflictADID = ""
		emp.ManagerConflictRaw = ""
		if err := q.UpdateEmployee(ctx, *emp); err != nil {
			return err
		}
		return s.audit(ctx, q, "", "", emp.ID, "manager_ad_changed", prevID, resolved)
	}
	emp.ManagerConflictADID = resolved
	emp.ManagerConflictRaw = raw
	if err := q.UpdateEmployee(ctx, *emp); err != nil {
		return err
	}
	after := resolved
	if after == "" {
		after = raw
	}
	if err := s.audit(ctx, q, "", "", emp.ID, "manager_ad_changed", prevID, after); err != nil {
		return err
	}
	return s.queueManagerADChanged(ctx, q, *emp, prevID, resolved, raw)
}

func (s *Service) queueManagerADChanged(ctx context.Context, q store.Queries, emp model.Employee, prevID, newID, raw string) error {
	hrs, err := s.hrPeople(ctx, q)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]string{
		"employee":         emp.DisplayName,
		"ad_manager":       s.managerLabel(ctx, q, newID, raw),
		"previous_manager": s.managerLabel(ctx, q, prevID, ""),
		"override_manager": s.managerLabel(ctx, q, emp.ManagerOverrideID, ""),
	})
	if err != nil {
		return err
	}
	dedupe := emp.ID + "|" + newID + "|" + raw
	now := s.now()
	for _, hr := range hrs {
		lang := hr.Locale
		if lang != "en" {
			lang = "ru"
		}
		if err := q.InsertNotification(ctx, model.Notification{
			ID: s.id(), RecipientEmployeeID: hr.ID, Kind: notify.KindManagerADChanged,
			RecipientEmail: hr.Email, Language: lang, DueAt: now, DedupeKey: dedupe,
			Payload: payload, CreatedAt: now,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) managerLabel(ctx context.Context, q store.Queries, id, raw string) string {
	if id != "" {
		if person, err := q.GetEmployee(ctx, id); err == nil && person.DisplayName != "" {
			return person.DisplayName
		}
	}
	return strings.TrimSpace(raw)
}

// ManagerDirectory lists every employee under the manager who approves their vacation.
func (s *Service) ManagerDirectory(ctx context.Context, p identity.Principal) (ManagerDirectory, error) {
	var out ManagerDirectory
	err := s.tx(ctx, func(q store.Queries) error {
		if _, err := s.requireHR(ctx, q, p); err != nil {
			return err
		}
		var err error
		out, err = s.managerDirectory(ctx, q)
		return err
	})
	return out, err
}

// SetManager records an HR choice about the manual manager.
// set stores a manual value. clear and take_ad remove it so the directory wins.
// keep leaves the manual value and dismisses the pending directory change.
func (s *Service) SetManager(ctx context.Context, p identity.Principal, employeeID string, in ManagerAction) (ManagerDirectory, error) {
	var out ManagerDirectory
	err := s.tx(ctx, func(q store.Queries) error {
		actor, err := s.requireHR(ctx, q, p)
		if err != nil {
			return err
		}
		emp, err := q.GetEmployee(ctx, employeeID)
		if err != nil {
			return missing(err)
		}
		action := strings.TrimSpace(in.Action)
		switch action {
		case "set":
			managerID := strings.TrimSpace(in.ManagerID)
			if managerID == "" || managerID == emp.ID {
				return code("validation", "choose a different manager", 400)
			}
			mgr, err := q.GetEmployee(ctx, managerID)
			if err != nil {
				return missing(err)
			}
			if !mgr.Active {
				return code("validation", "that manager is inactive", 400)
			}
			emp.ManagerOverrideID = mgr.ID
			emp.ManagerID = mgr.ID
			emp.ManagerConflictADID = ""
			emp.ManagerConflictRaw = ""
			if err := s.audit(ctx, q, actor.ID, "", emp.ID, "manager_override_set", "", mgr.ID); err != nil {
				return err
			}
		case "clear", "take_ad":
			emp.ManagerOverrideID = ""
			emp.ManagerID = emp.ADManagerID
			emp.ManagerConflictADID = ""
			emp.ManagerConflictRaw = ""
			name := "manager_override_cleared"
			if action == "take_ad" {
				name = "manager_ad_taken"
			}
			if err := s.audit(ctx, q, actor.ID, "", emp.ID, name, "", emp.ManagerID); err != nil {
				return err
			}
		case "keep":
			emp.ManagerConflictADID = ""
			emp.ManagerConflictRaw = ""
			if emp.ManagerOverrideID != "" {
				emp.ManagerID = emp.ManagerOverrideID
			}
			if err := s.audit(ctx, q, actor.ID, "", emp.ID, "manager_override_kept", "", emp.ManagerID); err != nil {
				return err
			}
		default:
			return code("validation", "action must be set, clear, take_ad, or keep", 400)
		}
		emp.UpdatedAt = s.now()
		if err := q.UpdateEmployee(ctx, emp); err != nil {
			return err
		}
		out, err = s.managerDirectory(ctx, q)
		return err
	})
	return out, err
}

func (s *Service) managerDirectory(ctx context.Context, q store.Queries) (ManagerDirectory, error) {
	all, err := q.ListEmployees(ctx)
	if err != nil {
		return ManagerDirectory{}, err
	}
	units, err := q.ListOrgUnits(ctx)
	if err != nil {
		return ManagerDirectory{}, err
	}
	byID := map[string]model.Employee{}
	for _, e := range all {
		byID[e.ID] = e
	}
	unitName := map[string]string{}
	for _, u := range units {
		unitName[u.ID] = u.Name
	}
	grouped := map[string][]ManagedEmployee{}
	for _, e := range all {
		key := e.ManagerID
		if key == e.ID {
			key = ""
		}
		if _, ok := byID[key]; key != "" && !ok {
			key = ""
		}
		grouped[key] = append(grouped[key], managedEmployee(e, byID, unitName))
	}
	out := ManagerDirectory{Groups: []ManagerGroup{}}
	for key, people := range grouped {
		sort.Slice(people, func(i, j int) bool {
			if people[i].Name == people[j].Name {
				return people[i].ID < people[j].ID
			}
			return people[i].Name < people[j].Name
		})
		group := ManagerGroup{People: people}
		if key != "" {
			mgr := byID[key]
			group.Manager = &Ref{ID: mgr.ID, Name: mgr.DisplayName}
		}
		out.Groups = append(out.Groups, group)
	}
	sort.Slice(out.Groups, func(i, j int) bool {
		a, b := out.Groups[i].Manager, out.Groups[j].Manager
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		if a.Name == b.Name {
			return a.ID < b.ID
		}
		return a.Name < b.Name
	})
	return out, nil
}

func managedEmployee(e model.Employee, byID map[string]model.Employee, unitName map[string]string) ManagedEmployee {
	out := ManagedEmployee{
		ID: e.ID, Name: e.DisplayName, Email: e.Email, EmployeeNumber: e.EmployeeNumber,
		Position: e.Position, Department: unitName[e.DepartmentID], Active: e.Active,
		Manual: e.ManagerOverrideID != "", ADManagerRaw: e.ManagerRaw,
	}
	if m, ok := byID[e.ManagerID]; ok && e.ManagerID != "" && e.ManagerID != e.ID {
		out.Manager = &Ref{ID: m.ID, Name: m.DisplayName}
	}
	if m, ok := byID[e.ADManagerID]; ok && e.ADManagerID != "" {
		out.ADManager = &Ref{ID: m.ID, Name: m.DisplayName}
	}
	if m, ok := byID[e.ManagerOverrideID]; ok && e.ManagerOverrideID != "" {
		out.Override = &Ref{ID: m.ID, Name: m.DisplayName}
	}
	if e.ManagerConflictADID != "" || e.ManagerConflictRaw != "" {
		if m, ok := byID[e.ManagerConflictADID]; ok && e.ManagerConflictADID != "" {
			out.ADChange = &Ref{ID: m.ID, Name: m.DisplayName}
		}
		out.ADChangeRaw = e.ManagerConflictRaw
	}
	return out
}
