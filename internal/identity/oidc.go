package identity

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Validator checks access tokens against a Keycloak realm's JWKS.
type Validator struct {
	Issuer   string
	ClientID string
	HTTP     *http.Client
	Now      func() time.Time

	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

// Authenticator is what the HTTP layer calls.
type Authenticator interface {
	Authenticate(r *http.Request) (Principal, error)
}

func (v *Validator) client() *http.Client {
	if v.HTTP != nil {
		return v.HTTP
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (v *Validator) now() time.Time {
	if v.Now != nil {
		return v.Now()
	}
	return time.Now()
}

// Authenticate reads a Bearer token and validates it.
func (v *Validator) Authenticate(r *http.Request) (Principal, error) {
	raw := bearer(r.Header.Get("Authorization"))
	if raw == "" {
		return Principal{}, errors.New("missing bearer token")
	}
	return v.Validate(r.Context(), raw)
}

func bearer(h string) string {
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// Validate checks signature, issuer, expiry, and audience or authorized party.
func (v *Validator) Validate(ctx context.Context, raw string) (Principal, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(strings.TrimRight(v.Issuer, "/")),
		jwt.WithTimeFunc(v.now),
	)
	var claims jwt.MapClaims
	tok, err := parser.ParseWithClaims(raw, &claims, func(tok *jwt.Token) (any, error) {
		kid, _ := tok.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("token has no kid")
		}
		return v.key(ctx, kid)
	})
	if err != nil || !tok.Valid {
		return Principal{}, fmt.Errorf("token rejected")
	}
	if !audienceOK(claims, v.ClientID) {
		return Principal{}, errors.New("token audience rejected")
	}
	return principalFromClaims(claims), nil
}

func audienceOK(claims jwt.MapClaims, clientID string) bool {
	if clientID == "" {
		return false
	}
	if azp, _ := claims["azp"].(string); azp == clientID {
		return true
	}
	switch aud := claims["aud"].(type) {
	case string:
		return aud == clientID
	case []any:
		for _, item := range aud {
			if s, ok := item.(string); ok && s == clientID {
				return true
			}
		}
	}
	return false
}

func principalFromClaims(claims jwt.MapClaims) Principal {
	p := Principal{Claims: map[string]string{}}
	if sub, _ := claims["sub"].(string); sub != "" {
		p.Subject = sub
	}
	if name, _ := claims["name"].(string); name != "" {
		p.Name = name
	}
	if p.Name == "" {
		if u, _ := claims["preferred_username"].(string); u != "" {
			p.Name = u
		}
	}
	if email, _ := claims["email"].(string); email != "" {
		p.Email = email
	}
	for k, v := range claims {
		switch t := v.(type) {
		case string:
			p.Claims[k] = t
		}
	}
	p.Groups = stringList(claims["groups"])
	if len(p.Groups) == 0 {
		p.Groups = stringList(claims["group"])
	}
	return p
}

func stringList(v any) []string {
	switch t := v.(type) {
	case []any:
		var out []string
		for _, item := range t {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if t == "" {
			return nil
		}
		return strings.FieldsFunc(t, func(r rune) bool { return r == ',' || r == ' ' })
	default:
		return nil
	}
}

func (v *Validator) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.keys != nil && time.Since(v.fetched) < time.Hour {
		if k, ok := v.keys[kid]; ok {
			return k, nil
		}
	}
	keys, err := v.fetch(ctx)
	if err != nil {
		return nil, err
	}
	v.keys = keys
	v.fetched = time.Now()
	k, ok := keys[kid]
	if !ok {
		return nil, errors.New("signing key not found")
	}
	return k, nil
}

type oidcDoc struct {
	JWKSURI string `json:"jwks_uri"`
}

type jwksDoc struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
	Use string `json:"use"`
}

func (v *Validator) fetch(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	issuer := strings.TrimRight(v.Issuer, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuer+"/.well-known/openid-configuration", nil)
	if err != nil {
		return nil, err
	}
	res, err := v.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openid configuration status %d", res.StatusCode)
	}
	var doc oidcDoc
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		return nil, err
	}
	if doc.JWKSURI == "" {
		return nil, errors.New("jwks_uri missing")
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, doc.JWKSURI, nil)
	if err != nil {
		return nil, err
	}
	res2, err := v.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks status %d", res2.StatusCode)
	}
	var set jwksDoc
	if err := json.NewDecoder(res2.Body).Decode(&set); err != nil {
		return nil, err
	}
	out := map[string]*rsa.PublicKey{}
	for _, k := range set.Keys {
		if k.Kty != "RSA" || k.Kid == "" || (k.Use != "" && k.Use != "sig") {
			continue
		}
		pub, err := rsaKey(k)
		if err != nil {
			continue
		}
		out[k.Kid] = pub
	}
	if len(out) == 0 {
		return nil, errors.New("no signing keys")
	}
	return out, nil
}

func rsaKey(k jwk) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, err
	}
	eb, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, err
	}
	e := 0
	for _, b := range eb {
		e = e<<8 + int(b)
	}
	if e == 0 {
		return nil, errors.New("bad exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: e}, nil
}
