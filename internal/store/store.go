// Package store is the Postgres access used by the API and the worker.
// Memory is the in-process implementation used by tests.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/wgh-control/bkl-vacation/internal/model"
)

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("not found")

// Queries is the set of reads and writes inside one transaction.
type Queries interface {
	UpsertEmployee(ctx context.Context, e model.Employee) (model.Employee, error)
	UpdateEmployee(ctx context.Context, e model.Employee) error
	GetEmployee(ctx context.Context, id string) (model.Employee, error)
	GetEmployeeBySubject(ctx context.Context, subject string) (model.Employee, error)
	ListEmployees(ctx context.Context) ([]model.Employee, error)

	UpsertOrgUnit(ctx context.Context, u model.OrgUnit) (model.OrgUnit, error)
	GetOrgUnit(ctx context.Context, id string) (model.OrgUnit, error)
	ListOrgUnits(ctx context.Context) ([]model.OrgUnit, error)

	UpsertLeaveYear(ctx context.Context, y model.LeaveYear) (model.LeaveYear, error)
	ListLeaveYears(ctx context.Context, employeeID string) ([]model.LeaveYear, error)
	LockLeaveYear(ctx context.Context, id string) error

	InsertRequest(ctx context.Context, r model.Request) error
	UpdateRequest(ctx context.Context, r model.Request) error
	GetRequest(ctx context.Context, id string) (model.Request, error)
	ListRequests(ctx context.Context, f model.RequestFilter) ([]model.Request, error)

	InsertStep(ctx context.Context, s model.Step) error
	UpdateStep(ctx context.Context, s model.Step) error
	ListSteps(ctx context.Context, requestID string) ([]model.Step, error)

	UpsertCalendarDay(ctx context.Context, d model.CalendarDay) error
	ListCalendarDays(ctx context.Context, from, to time.Time) ([]model.CalendarDay, error)

	InsertNotification(ctx context.Context, n model.Notification) error
	ListUnsentDue(ctx context.Context, now time.Time) ([]model.Notification, error)
	UpdateNotification(ctx context.Context, n model.Notification) error

	InsertNotice(ctx context.Context, n model.Notice) error
	ListNotices(ctx context.Context, employeeID string) ([]model.Notice, error)
	MarkNoticeRead(ctx context.Context, id, employeeID string, at time.Time) error

	InsertAudit(ctx context.Context, a model.Audit) error

	UpsertScheduleLock(ctx context.Context, lock model.ScheduleLock) error
	GetScheduleLock(ctx context.Context, year int) (model.ScheduleLock, error)
}

// Repo opens transactions and the worker lock.
type Repo interface {
	View(ctx context.Context, fn func(Queries) error) error
	RunInTx(ctx context.Context, fn func(Queries) error) error
	WithAdvisoryLock(ctx context.Context, fn func(context.Context) error) (bool, error)
	Ping(ctx context.Context) error
}
