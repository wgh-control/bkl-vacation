// Package worker runs directory sync and notification delivery.
package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/wgh-control/bkl-vacation/internal/notify"
	"github.com/wgh-control/bkl-vacation/internal/org"
	"github.com/wgh-control/bkl-vacation/internal/portal"
	"github.com/wgh-control/bkl-vacation/internal/store"
)

// Worker is the second process. A Postgres advisory lock keeps a second replica from double-sending.
type Worker struct {
	Svc         *portal.Service
	Repo        store.Repo
	Directory   org.Directory
	Mail        notify.Sender
	SyncEvery   time.Duration
	NotifyEvery time.Duration
	Log         *slog.Logger
}

func (w *Worker) log() *slog.Logger {
	if w.Log != nil {
		return w.Log
	}
	return slog.Default()
}

// Run syncs and sends until the context is cancelled. It also runs once immediately.
func (w *Worker) Run(ctx context.Context) {
	w.sync(ctx)
	w.notify(ctx)
	syncEvery := w.SyncEvery
	if syncEvery <= 0 {
		syncEvery = 15 * time.Minute
	}
	notifyEvery := w.NotifyEvery
	if notifyEvery <= 0 {
		notifyEvery = time.Minute
	}
	syncTick := time.NewTicker(syncEvery)
	noteTick := time.NewTicker(notifyEvery)
	defer syncTick.Stop()
	defer noteTick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-syncTick.C:
			w.sync(ctx)
		case <-noteTick.C:
			w.notify(ctx)
		}
	}
}

func (w *Worker) sync(ctx context.Context) {
	if w.Directory == nil {
		return
	}
	ok, err := w.Repo.WithAdvisoryLock(ctx, func(ctx context.Context) error {
		users, err := w.Directory.ListUsers(ctx)
		if err != nil {
			return err
		}
		mapped := make([]portal.DirectoryUser, 0, len(users))
		for _, u := range users {
			mapped = append(mapped, portal.DirectoryUser{
				Subject: u.Subject, DisplayName: u.DisplayName, Email: u.Email, Locale: u.Locale,
				EmployeeNumber: u.EmployeeNumber, Position: u.Position, Department: u.Department,
				ManagerRaw: u.ManagerRaw, HireDate: u.HireDate, Active: u.Active, HR: u.HR, Raw: u.Raw,
			})
		}
		return w.Svc.ApplyDirectory(ctx, mapped)
	})
	if err != nil {
		w.log().Error("directory sync", "err", err.Error())
		return
	}
	if !ok {
		w.log().Info("directory sync skipped; another worker holds the lock")
	}
}

func (w *Worker) notify(ctx context.Context) {
	ok, err := w.Repo.WithAdvisoryLock(ctx, func(ctx context.Context) error {
		return w.Svc.Dispatch(ctx, w.Mail)
	})
	if err != nil {
		w.log().Error("notifications", "err", err.Error())
		return
	}
	if !ok {
		w.log().Info("notifications skipped; another worker holds the lock")
	}
}
