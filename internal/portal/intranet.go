package portal

import (
	"context"

	"github.com/wgh-control/bkl-vacation/internal/identity"
)

// Intranet is the OpenIntranet (Drupal) link. It names the Keycloak person
// already stored on the employee row, the modules that person can open, and
// portal paths. It does not open another database.
type Intranet struct {
	User    IntranetUser     `json:"user"`
	Modules []IntranetModule `json:"modules"`
}

// IntranetUser is the same person as the access token. Subject is the Keycloak sub.
type IntranetUser struct {
	Subject        string `json:"subject"`
	ID             string `json:"id"`
	DisplayName    string `json:"display_name"`
	Email          string `json:"email,omitempty"`
	Locale         string `json:"locale"`
	EmployeeNumber string `json:"employee_number,omitempty"`
	Active         bool   `json:"active"`
}

// IntranetModule is one area of the existing portal.
type IntranetModule struct {
	ID    string         `json:"id"`
	Links []IntranetLink `json:"links"`
}

// IntranetLink is a portal UI path. It is not an /api route.
// A request link uses {id} for a vacation request id.
type IntranetLink struct {
	Rel  string `json:"rel"`
	Path string `json:"path"`
}

// Intranet returns who the caller is and which portal modules they can open.
// The employee module is open to every signed-in person. The manager module is
// open when they have direct reports. The HR module is open for the HR group.
func (s *Service) Intranet(ctx context.Context, p identity.Principal) (Intranet, error) {
	profile, err := s.Me(ctx, p)
	if err != nil {
		return Intranet{}, err
	}
	out := Intranet{
		User: IntranetUser{
			Subject:        profile.Subject,
			ID:             profile.ID,
			DisplayName:    profile.DisplayName,
			Email:          profile.Email,
			Locale:         profile.Locale,
			EmployeeNumber: profile.EmployeeNumber,
			Active:         profile.Active,
		},
		Modules: []IntranetModule{},
	}
	if !profile.Active {
		return out, nil
	}
	out.Modules = append(out.Modules, IntranetModule{
		ID: "employee",
		Links: []IntranetLink{
			{Rel: "open", Path: "/"},
			{Rel: "new", Path: "/requests/new"},
			{Rel: "request", Path: "/requests/{id}"},
		},
	})
	if profile.ManagerOfOthers {
		out.Modules = append(out.Modules, IntranetModule{
			ID: "manager",
			Links: []IntranetLink{
				{Rel: "open", Path: "/team"},
			},
		})
	}
	if profile.HR {
		out.Modules = append(out.Modules, IntranetModule{
			ID:    "hr",
			Links: []IntranetLink{{Rel: "open", Path: "/hr"}, {Rel: "reports", Path: "/hr/reports"}},
		})
	}
	return out, nil
}
