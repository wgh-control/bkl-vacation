package vacation

import (
	"embed"
	"io/fs"
	"sort"
	"strings"

	"github.com/wgh-control/bkl-vacation/internal/config"
	"github.com/wgh-control/bkl-vacation/internal/notify"
	"github.com/wgh-control/bkl-vacation/internal/platform"
	"github.com/wgh-control/bkl-vacation/internal/portal"
	"github.com/wgh-control/bkl-vacation/internal/store"
)

//go:embed schema/*.sql
var schemaFS embed.FS

// Module is the vacation area on the shared API and database.
type Module struct {
	Svc *portal.Service
}

// New builds the vacation module. Mail may be nil when SMTP is not configured.
func New(repo store.Repo, hrGroup string, claims config.ClaimNames, mail notify.Sender, financeEmail string) *Module {
	svc := portal.New(repo, hrGroup, claims)
	svc.Mail = mail
	svc.FinanceEmail = financeEmail
	return &Module{Svc: svc}
}

func (m *Module) Name() string { return "vacation" }

func (m *Module) Tables() []platform.Table { return Tables() }

func (m *Module) Routes(r platform.Router) {
	if m == nil || m.Svc == nil {
		return
	}
	mount(m.Svc, r)
}

func (m *Module) Mail() platform.Mailer {
	if m == nil || m.Svc == nil {
		return nil
	}
	return m.Svc
}

// Tables returns vacation's SQL in apply order.
// The version key is the file name, matching rows already stored in schema_migration.
func Tables() []platform.Table {
	names, err := fs.Glob(schemaFS, "schema/*.sql")
	if err != nil {
		panic(err)
	}
	sort.Strings(names)
	out := make([]platform.Table, 0, len(names))
	for _, name := range names {
		body, err := schemaFS.ReadFile(name)
		if err != nil {
			panic(err)
		}
		out = append(out, platform.Table{
			Version: strings.TrimPrefix(name, "schema/"),
			SQL:     string(body),
		})
	}
	return out
}
