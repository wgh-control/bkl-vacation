package org

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wgh-control/bkl-vacation/internal/config"
)

// Directory lists people from the identity provider.
type Directory interface {
	ListUsers(ctx context.Context) ([]User, error)
}

// Keycloak reads the existing realm's Admin API. It does not stand up a provider.
type Keycloak struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	HRGroup      string
	Claims       config.ClaimNames
	HTTP         *http.Client
}

type kcAccount struct {
	ID         string              `json:"id"`
	Username   string              `json:"username"`
	Email      string              `json:"email"`
	FirstName  string              `json:"firstName"`
	LastName   string              `json:"lastName"`
	Enabled    bool                `json:"enabled"`
	Attributes map[string][]string `json:"attributes"`
}

type kcGroup struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (k *Keycloak) client() *http.Client {
	if k.HTTP != nil {
		return k.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// ParseIssuer splits a realm issuer URL into the server origin and the realm name.
func ParseIssuer(issuer string) (origin, realm string, err error) {
	u, err := url.Parse(strings.TrimSpace(issuer))
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "realms" && parts[i+1] != "" {
			u.Path = ""
			u.RawQuery = ""
			u.Fragment = ""
			return strings.TrimRight(u.String(), "/"), parts[i+1], nil
		}
	}
	return "", "", fmt.Errorf("issuer URL has no realm")
}

// ListUsers pages through the Admin API and resolves group membership.
func (k *Keycloak) ListUsers(ctx context.Context) ([]User, error) {
	origin, realm, err := ParseIssuer(k.Issuer)
	if err != nil {
		return nil, err
	}
	token, err := k.token(ctx, origin, realm)
	if err != nil {
		return nil, err
	}
	var users []User
	for first := 0; ; first += 100 {
		endpoint := fmt.Sprintf("%s/admin/realms/%s/users?first=%d&max=100", origin, url.PathEscape(realm), first)
		var page []kcAccount
		if err := k.getJSON(ctx, token, endpoint, &page); err != nil {
			return nil, err
		}
		for _, account := range page {
			groups, err := k.groups(ctx, token, origin, realm, account.ID)
			if err != nil {
				return nil, err
			}
			kc := KCUser{
				ID: account.ID, Username: account.Username, Email: account.Email,
				FirstName: account.FirstName, LastName: account.LastName, Enabled: account.Enabled,
				Attributes: account.Attributes, Groups: groups,
			}
			users = append(users, MapUser(kc, k.HRGroup, k.Claims))
		}
		if len(page) < 100 {
			break
		}
	}
	return users, nil
}

func (k *Keycloak) groups(ctx context.Context, token, origin, realm, id string) ([]string, error) {
	endpoint := fmt.Sprintf("%s/admin/realms/%s/users/%s/groups", origin, url.PathEscape(realm), url.PathEscape(id))
	var groups []kcGroup
	if err := k.getJSON(ctx, token, endpoint, &groups); err != nil {
		return nil, err
	}
	var out []string
	for _, g := range groups {
		if g.Path != "" {
			out = append(out, g.Path)
		} else if g.Name != "" {
			out = append(out, g.Name)
		}
	}
	return out, nil
}

func (k *Keycloak) token(ctx context.Context, origin, realm string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", k.ClientID)
	form.Set("client_secret", k.ClientSecret)
	endpoint := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", origin, url.PathEscape(realm))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := k.client().Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("keycloak token status %d", res.StatusCode)
	}
	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.AccessToken == "" {
		return "", fmt.Errorf("keycloak token response had no access_token")
	}
	return payload.AccessToken, nil
}

func (k *Keycloak) getJSON(ctx context.Context, token, endpoint string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := k.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("keycloak status %d", res.StatusCode)
	}
	return json.Unmarshal(body, dest)
}
