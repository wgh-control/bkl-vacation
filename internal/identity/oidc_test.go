package identity

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestValidateAccessToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks := jwksDoc{Keys: []jwk{{
		Kty: "RSA",
		Kid: "k1",
		Use: "sig",
		N:   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}}}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/realms/bkl/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(oidcDoc{JWKSURI: srv.URL + "/jwks"})
		case "/jwks":
			_ = json.NewEncoder(w).Encode(jwks)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":             srv.URL + "/realms/bkl",
		"sub":             "user-1",
		"aud":             "account",
		"azp":             "vacation-portal",
		"exp":             time.Now().Add(time.Hour).Unix(),
		"iat":             time.Now().Unix(),
		"name":            "Ada",
		"email":           "ada@example.com",
		"groups":          []string{"/hr"},
		"employee_number": "100",
	})
	token.Header["kid"] = "k1"
	raw, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	v := &Validator{
		Issuer:   srv.URL + "/realms/bkl",
		ClientID: "vacation-portal",
		HTTP:     srv.Client(),
	}
	p, err := v.Validate(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if p.Subject != "user-1" || p.Email != "ada@example.com" || p.Name != "Ada" {
		t.Fatalf("principal %+v", p)
	}
	if !p.HasGroup("hr") {
		t.Fatal("hr group")
	}
	if p.Claim("employee_number") != "100" {
		t.Fatalf("claim %q", p.Claim("employee_number"))
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	if _, err := v.Authenticate(req); err != nil {
		t.Fatal(err)
	}
	other := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": srv.URL + "/realms/bkl",
		"sub": "user-2",
		"aud": "something-else",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	other.Header["kid"] = "k1"
	rawOther, err := other.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Validate(context.Background(), rawOther); err == nil {
		t.Fatal("wrong audience must be rejected")
	}
}
