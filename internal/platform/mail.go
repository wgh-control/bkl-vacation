package platform

import (
	"context"
	"fmt"

	"github.com/wgh-control/bkl-vacation/internal/notify"
)

// DispatchAll sends each module's mail with the shared sender.
// A nil module or a nil mailer is skipped. The caller holds the worker lock
// so one pass cannot run twice.
func DispatchAll(ctx context.Context, modules []Module, sender notify.Sender) error {
	for _, m := range modules {
		if m == nil || m.Mail() == nil {
			continue
		}
		if err := m.Mail().Dispatch(ctx, sender); err != nil {
			return fmt.Errorf("%s: %w", m.Name(), err)
		}
	}
	return nil
}
