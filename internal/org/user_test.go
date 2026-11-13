package org

import (
	"testing"

	"github.com/wgh-control/bkl-vacation/internal/config"
)

func TestMapUser(t *testing.T) {
	claims := config.ClaimNames{
		Email: "email", Locale: "locale", EmployeeNumber: "employeeNumber",
		Position: "title", Department: "department", Manager: "manager", HireDate: "hireDate",
	}
	u := MapUser(KCUser{
		ID: "sub-1", Username: "ada", FirstName: "Ada", LastName: "Lovelace", Enabled: true,
		Attributes: map[string][]string{
			"employeeNumber": {"42"},
			"title":          {"Engineer"},
			"department":     {"Engineering/Platform"},
			"manager":        {"sub-mgr"},
			"hireDate":       {"2020-03-15"},
			"locale":         {"en"},
		},
		Groups: []string{"/company/hr"},
	}, "hr", claims)
	if u.DisplayName != "Ada Lovelace" || u.EmployeeNumber != "42" || u.Position != "Engineer" {
		t.Fatalf("%+v", u)
	}
	if u.Department != "Engineering/Platform" || u.ManagerRaw != "sub-mgr" || !u.HR || !u.Active {
		t.Fatalf("%+v", u)
	}
	if u.HireDate == nil || u.HireDate.Format("2006-01-02") != "2020-03-15" || u.Locale != "en" {
		t.Fatalf("hire/locale %+v", u)
	}
	ext, name, parent := DepartmentPath(u.Department)
	if ext != "Engineering/Platform" || name != "Platform" || parent != "Engineering" {
		t.Fatalf("path %s %s %s", ext, name, parent)
	}
}

func TestDisabledUserStays(t *testing.T) {
	u := MapUser(KCUser{ID: "sub-2", Username: "bea", Enabled: false}, "hr", config.ClaimNames{})
	if u.Active {
		t.Fatal("disabled directory accounts are inactive, not deleted")
	}
	if u.DisplayName != "bea" {
		t.Fatalf("name %s", u.DisplayName)
	}
}
