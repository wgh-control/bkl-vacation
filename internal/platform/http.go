package platform

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/wgh-control/bkl-vacation/internal/identity"
)

// Deps is the shared HTTP host. Modules add their own routes.
type Deps struct {
	Auth    identity.Authenticator
	Ready   func(context.Context) error
	CORS    string
	OpenAPI []byte
	Modules []Module
}

// Handler serves health, readiness, and OpenAPI, then every module route.
func Handler(d Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/healthz", health)
	mux.HandleFunc("GET /api/readyz", ready(d.Ready))
	mux.HandleFunc("GET /api/openapi.yaml", openAPI(d.OpenAPI))
	rt := &router{mux: mux, auth: d.Auth}
	for _, m := range d.Modules {
		if m == nil {
			continue
		}
		m.Routes(rt)
	}
	return cors(d.CORS, mux)
}

type router struct {
	mux  *http.ServeMux
	auth identity.Authenticator
}

func (rt *router) Handle(pattern string, h http.HandlerFunc) {
	rt.mux.HandleFunc(pattern, h)
}

func (rt *router) Authenticated(pattern string, h http.HandlerFunc) {
	rt.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if rt.auth == nil {
			WriteJSON(w, http.StatusUnauthorized, map[string]string{
				"code": "unauthorized", "message": "authentication is not configured",
			})
			return
		}
		p, err := rt.auth.Authenticate(r)
		if err != nil || p.Subject == "" {
			WriteJSON(w, http.StatusUnauthorized, map[string]string{
				"code": "unauthorized", "message": "bearer token was rejected",
			})
			return
		}
		h(w, r.WithContext(WithPrincipal(r.Context(), p)))
	})
}

func health(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func ready(ping func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ping != nil {
			if err := ping(r.Context()); err != nil {
				WriteJSON(w, http.StatusServiceUnavailable, map[string]string{
					"code": "unavailable", "message": "database is not reachable",
				})
				return
			}
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

func openAPI(body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(body)
	}
}

func cors(origin string, next http.Handler) http.Handler {
	if origin == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// WriteJSON writes a JSON body with the shared content type.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
