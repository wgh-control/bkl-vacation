// Package org maps Keycloak users, which themselves come from Active Directory,
// into the portal's directory records.
package org

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/wgh-control/bkl-vacation/internal/config"
)

// User is one directory account ready to upsert.
type User struct {
	Subject        string
	Username       string
	DisplayName    string
	Email          string
	Locale         string
	EmployeeNumber string
	Position       string
	Department     string
	ManagerRaw     string
	HireDate       *time.Time
	Active         bool
	HR             bool
	Raw            []byte
}

// KCUser is the subset of a Keycloak user representation the sync reads.
type KCUser struct {
	ID         string
	Username   string
	Email      string
	FirstName  string
	LastName   string
	Enabled    bool
	Attributes map[string][]string
	Groups     []string
}

// MapUser copies the attributes Keycloak actually provides. Unknown claims stay empty.
func MapUser(kc KCUser, hrGroup string, claims config.ClaimNames) User {
	u := User{
		Subject:        kc.ID,
		Username:       kc.Username,
		Email:          firstNonEmpty(kc.Email, attr(kc.Attributes, claims.Email)),
		Locale:         attr(kc.Attributes, claims.Locale),
		EmployeeNumber: attr(kc.Attributes, claims.EmployeeNumber),
		Position:       attr(kc.Attributes, claims.Position),
		Department:     attr(kc.Attributes, claims.Department),
		ManagerRaw:     attr(kc.Attributes, claims.Manager),
		Active:         kc.Enabled,
		HR:             inGroup(kc.Groups, hrGroup),
	}
	u.DisplayName = strings.TrimSpace(kc.FirstName + " " + kc.LastName)
	if u.DisplayName == "" {
		u.DisplayName = firstNonEmpty(kc.Username, kc.Email, kc.ID)
	}
	if hire := attr(kc.Attributes, claims.HireDate); hire != "" {
		if t, ok := parseHire(hire); ok {
			u.HireDate = &t
		}
	}
	if raw, err := json.Marshal(kc.Attributes); err == nil {
		u.Raw = raw
	}
	return u
}

func attr(m map[string][]string, key string) string {
	if key == "" || m == nil {
		return ""
	}
	vals := m[key]
	if len(vals) == 0 {
		return ""
	}
	return strings.TrimSpace(vals[0])
}

func inGroup(groups []string, name string) bool {
	if name == "" {
		return false
	}
	for _, g := range groups {
		g = strings.TrimSpace(g)
		if strings.EqualFold(g, name) {
			return true
		}
		if i := strings.LastIndex(g, "/"); i >= 0 && strings.EqualFold(g[i+1:], name) {
			return true
		}
	}
	return false
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func parseHire(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006-01-02", time.RFC3339, "02.01.2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			y, m, d := t.Date()
			return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), true
		}
	}
	return time.Time{}, false
}

// DepartmentPath splits a directory department into a leaf name and a parent path.
func DepartmentPath(path string) (external, name, parent string) {
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" {
		return "", "", ""
	}
	parts := strings.Split(path, "/")
	var clean []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			clean = append(clean, p)
		}
	}
	if len(clean) == 0 {
		return "", "", ""
	}
	name = clean[len(clean)-1]
	external = strings.Join(clean, "/")
	if len(clean) > 1 {
		parent = strings.Join(clean[:len(clean)-1], "/")
	}
	return external, name, parent
}
