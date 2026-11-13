package org

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wgh-control/bkl-vacation/internal/config"
)

func TestKeycloakListUsers(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/realms/bkl/protocol/openid-connect/token":
			if r.FormValue("client_id") == "" && r.PostFormValue("client_id") == "" {
				_ = r.ParseForm()
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"tok"}`))
		case r.URL.Path == "/admin/realms/bkl/users":
			if r.Header.Get("Authorization") != "Bearer tok" {
				http.Error(w, "no", http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`[{"id":"u1","username":"ada","firstName":"Ada","lastName":"L","enabled":true,"email":"ada@example.com","attributes":{"employee_number":["7"],"hire_date":["2019-01-02"]}}]`))
		case r.URL.Path == "/admin/realms/bkl/users/u1/groups":
			_, _ = w.Write([]byte(`[{"name":"hr","path":"/hr"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	k := &Keycloak{
		Issuer: srv.URL + "/realms/bkl", ClientID: "sync", ClientSecret: "secret", HRGroup: "hr",
		Claims: config.ClaimNames{EmployeeNumber: "employee_number", HireDate: "hire_date"},
		HTTP:   srv.Client(),
	}
	users, err := k.ListUsers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Subject != "u1" || !users[0].HR || users[0].EmployeeNumber != "7" {
		t.Fatalf("%+v", users)
	}
}

func TestParseIssuer(t *testing.T) {
	origin, realm, err := ParseIssuer("https://id.example/realms/bkl")
	if err != nil || origin != "https://id.example" || realm != "bkl" {
		t.Fatalf("%s %s %v", origin, realm, err)
	}
}
