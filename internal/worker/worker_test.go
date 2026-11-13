package worker

import (
	"context"
	"testing"

	"github.com/wgh-control/bkl-vacation/internal/notify"
	"github.com/wgh-control/bkl-vacation/internal/platform"
	"github.com/wgh-control/bkl-vacation/internal/store"
)

type countingModule struct {
	name string
	n    int
}

func (m *countingModule) Name() string { return m.name }
func (m *countingModule) Tables() []platform.Table {
	return nil
}
func (m *countingModule) Routes(platform.Router) {}
func (m *countingModule) Mail() platform.Mailer  { return m }
func (m *countingModule) Dispatch(context.Context, notify.Sender) error {
	m.n++
	return nil
}

func TestNotifyDispatchesEachModule(t *testing.T) {
	vacationMail := &countingModule{name: "vacation"}
	educationMail := &countingModule{name: "education"}
	w := &Worker{
		Repo:    store.NewMemory(),
		Modules: []platform.Module{vacationMail, educationMail},
	}
	w.notify(context.Background())
	if vacationMail.n != 1 || educationMail.n != 1 {
		t.Fatalf("vacation %d education %d", vacationMail.n, educationMail.n)
	}
}
