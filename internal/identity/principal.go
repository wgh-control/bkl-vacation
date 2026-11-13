package identity

import "strings"

// Principal is the person on a validated Keycloak access token.
type Principal struct {
	Subject string
	Name    string
	Email   string
	Groups  []string
	Claims  map[string]string
}

// HasGroup reports membership in a Keycloak group. A path such as "/hr" matches "hr".
func (p Principal) HasGroup(name string) bool {
	if name == "" {
		return false
	}
	for _, g := range p.Groups {
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

// Claim returns a flattened token claim. Missing claims are empty.
func (p Principal) Claim(name string) string {
	if p.Claims == nil {
		return ""
	}
	return strings.TrimSpace(p.Claims[name])
}
