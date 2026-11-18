// Package app is the process list of modules.
// Add a module here so its routes, tables, and mail run on the shared API and database.
package app

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wgh-control/bkl-vacation/internal/config"
	"github.com/wgh-control/bkl-vacation/internal/notify"
	"github.com/wgh-control/bkl-vacation/internal/platform"
	"github.com/wgh-control/bkl-vacation/internal/portal"
	"github.com/wgh-control/bkl-vacation/internal/store"
	"github.com/wgh-control/bkl-vacation/internal/vacation"
)

// Services are the shared dependencies handed to every module.
type Services struct {
	Repo         store.Repo
	Mail         notify.Sender
	HRGroup      string
	Claims       config.ClaimNames
	FinanceEmail string
}

// NewServices builds module dependencies from process config.
// Mail is nil until SMTP_HOST and SMTP_FROM are set.
func NewServices(cfg config.Config, repo store.Repo) Services {
	return Services{
		Repo:         repo,
		Mail:         Mail(cfg),
		HRGroup:      cfg.KeycloakHRGroup,
		Claims:       cfg.Claims,
		FinanceEmail: cfg.FinanceEmail,
	}
}

// Mail is the shared SMTP sender, or nil when mail is not configured.
func Mail(cfg config.Config) notify.Sender {
	if cfg.SMTPHost == "" || cfg.SMTPFrom == "" {
		return nil
	}
	return notify.SMTP{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort, Mode: cfg.SMTPTLS,
		User: cfg.SMTPUser, Password: cfg.SMTPPassword, From: cfg.SMTPFrom,
	}
}

// entry is one registered area. tables is its SQL. open builds the module
// that mounts routes and mail. Append an entry for a new area, such as
// education requests.
type entry struct {
	tables func() []platform.Table
	open   func(Services) platform.Module
}

func catalog() []entry {
	return []entry{
		{
			tables: vacation.Tables,
			open: func(s Services) platform.Module {
				return vacation.New(s.Repo, s.HRGroup, s.Claims, s.Mail, s.FinanceEmail)
			},
		},
	}
}

// Modules returns the areas served by this process, in catalog order.
func Modules(s Services) []platform.Module {
	list := catalog()
	out := make([]platform.Module, 0, len(list))
	for _, e := range list {
		out = append(out, e.open(s))
	}
	return out
}

// Tables is every module migration, in the same order as Modules.
func Tables() []platform.Table {
	var all []platform.Table
	for _, e := range catalog() {
		if e.tables == nil {
			continue
		}
		all = append(all, e.tables()...)
	}
	return all
}

// Migrate applies every registered module's tables to the shared database.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	return platform.Apply(ctx, pool, Tables())
}

// Vacation returns the vacation service used for the shared directory sync.
func Vacation(mods []platform.Module) *portal.Service {
	for _, m := range mods {
		if v, ok := m.(*vacation.Module); ok && v != nil {
			return v.Svc
		}
	}
	return nil
}
