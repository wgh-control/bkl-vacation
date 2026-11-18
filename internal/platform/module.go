// Package platform is the shared API and Postgres host.
// Vacation is one module. A later area, such as education requests,
// implements Module and is added to the process list in internal/app.
// The host owns the HTTP server, the database pool, the migration ledger,
// and the SMTP sender. Modules register routes, tables, and mail.
package platform

import (
	"context"
	"net/http"

	"github.com/wgh-control/bkl-vacation/internal/notify"
)

// Module is one product area on the shared API and database.
type Module interface {
	// Name is a short stable id used in logs.
	Name() string
	// Tables are the SQL migrations this module owns.
	// Version is the schema_migration key and must be unique across modules.
	Tables() []Table
	// Routes mounts handlers on the shared mux.
	// Patterns use the net/http form, for example "GET /api/requests".
	Routes(Router)
	// Mail sends this module's messages. Nil means the module has no mail.
	Mail() Mailer
}

// Table is one migration. SQL may contain several statements.
type Table struct {
	Version string
	SQL     string
}

// Mailer sends one module's due mail through the shared sender.
// The sender is nil when SMTP is not configured. The module still stores
// its in-app notices and leaves the mail for a later pass.
type Mailer interface {
	Dispatch(ctx context.Context, sender notify.Sender) error
}

// Router registers routes on the shared mux.
// Authenticated requires a bearer token and stores the principal on the request.
type Router interface {
	Handle(pattern string, h http.HandlerFunc)
	Authenticated(pattern string, h http.HandlerFunc)
}
