package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wgh-control/bkl-vacation/internal/model"
)

const advisoryKey int64 = 0x424B4C01

// Postgres is the external database. The cluster does not run Postgres.
type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func (p *Postgres) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

func (p *Postgres) View(ctx context.Context, fn func(Queries) error) error {
	return fn(&pgQ{q: p.pool})
}

func (p *Postgres) RunInTx(ctx context.Context, fn func(Queries) error) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(&pgQ{q: tx}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Postgres) WithAdvisoryLock(ctx context.Context, fn func(context.Context) error) (bool, error) {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Release()
	var ok bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, advisoryKey).Scan(&ok); err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	defer conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, advisoryKey)
	return true, fn(ctx)
}

type rowQuery interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type pgQ struct{ q rowQuery }

func (s *pgQ) UpsertEmployee(ctx context.Context, e model.Employee) (model.Employee, error) {
	const q = `
INSERT INTO employee (
    id, keycloak_sub, display_name, email, locale, locale_chosen, employee_number, position,
    department_id, manager_id, manager_raw, hire_date, active, hr, extra_days, early_leave,
    raw_attributes, last_sync_at, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
ON CONFLICT (keycloak_sub) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    email = EXCLUDED.email,
    locale = EXCLUDED.locale,
    locale_chosen = EXCLUDED.locale_chosen,
    employee_number = EXCLUDED.employee_number,
    position = EXCLUDED.position,
    department_id = EXCLUDED.department_id,
    manager_id = EXCLUDED.manager_id,
    manager_raw = EXCLUDED.manager_raw,
    hire_date = EXCLUDED.hire_date,
    active = EXCLUDED.active,
    hr = EXCLUDED.hr,
    extra_days = EXCLUDED.extra_days,
    early_leave = EXCLUDED.early_leave,
    raw_attributes = EXCLUDED.raw_attributes,
    last_sync_at = EXCLUDED.last_sync_at,
    updated_at = EXCLUDED.updated_at
RETURNING ` + employeeCols
	row := s.q.QueryRow(ctx, q,
		e.ID, e.Subject, e.DisplayName, e.Email, emptyLocale(e.Locale), e.LocaleChosen, e.EmployeeNumber, e.Position,
		uuidArg(e.DepartmentID), uuidArg(e.ManagerID), e.ManagerRaw, datePtr(e.HireDate), e.Active, e.HR, e.ExtraDays, e.EarlyLeave,
		jsonArg(e.Raw), timePtr(e.LastSyncAt), e.CreatedAt.UTC(), e.UpdatedAt.UTC(),
	)
	return scanEmployee(row)
}

func (s *pgQ) UpdateEmployee(ctx context.Context, e model.Employee) error {
	tag, err := s.q.Exec(ctx, `
UPDATE employee SET
    keycloak_sub=$2, display_name=$3, email=$4, locale=$5, locale_chosen=$6, employee_number=$7,
    position=$8, department_id=$9, manager_id=$10, manager_raw=$11, hire_date=$12, active=$13, hr=$14,
    extra_days=$15, early_leave=$16, raw_attributes=$17, last_sync_at=$18, updated_at=$19
WHERE id=$1`,
		e.ID, e.Subject, e.DisplayName, e.Email, emptyLocale(e.Locale), e.LocaleChosen, e.EmployeeNumber,
		e.Position, uuidArg(e.DepartmentID), uuidArg(e.ManagerID), e.ManagerRaw, datePtr(e.HireDate), e.Active, e.HR,
		e.ExtraDays, e.EarlyLeave, jsonArg(e.Raw), timePtr(e.LastSyncAt), e.UpdatedAt.UTC(),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgQ) GetEmployee(ctx context.Context, id string) (model.Employee, error) {
	return scanEmployee(s.q.QueryRow(ctx, `SELECT `+employeeCols+` FROM employee WHERE id=$1`, id))
}

func (s *pgQ) GetEmployeeBySubject(ctx context.Context, subject string) (model.Employee, error) {
	return scanEmployee(s.q.QueryRow(ctx, `SELECT `+employeeCols+` FROM employee WHERE keycloak_sub=$1`, subject))
}

func (s *pgQ) ListEmployees(ctx context.Context) ([]model.Employee, error) {
	rows, err := s.q.Query(ctx, `SELECT `+employeeCols+` FROM employee ORDER BY display_name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Employee
	for rows.Next() {
		e, err := scanEmployee(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *pgQ) UpsertOrgUnit(ctx context.Context, u model.OrgUnit) (model.OrgUnit, error) {
	row := s.q.QueryRow(ctx, `
INSERT INTO org_unit (id, external_id, name, parent_id, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (external_id) DO UPDATE SET name=EXCLUDED.name, parent_id=EXCLUDED.parent_id, updated_at=EXCLUDED.updated_at
RETURNING id, external_id, name, parent_id::text, created_at, updated_at`,
		u.ID, u.ExternalID, u.Name, uuidArg(u.ParentID), u.CreatedAt.UTC(), u.UpdatedAt.UTC())
	return scanUnit(row)
}

func (s *pgQ) GetOrgUnit(ctx context.Context, id string) (model.OrgUnit, error) {
	return scanUnit(s.q.QueryRow(ctx, `SELECT id, external_id, name, parent_id::text, created_at, updated_at FROM org_unit WHERE id=$1`, id))
}

func (s *pgQ) ListOrgUnits(ctx context.Context) ([]model.OrgUnit, error) {
	rows, err := s.q.Query(ctx, `SELECT id, external_id, name, parent_id::text, created_at, updated_at FROM org_unit ORDER BY external_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.OrgUnit
	for rows.Next() {
		u, err := scanUnit(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *pgQ) UpsertLeaveYear(ctx context.Context, y model.LeaveYear) (model.LeaveYear, error) {
	row := s.q.QueryRow(ctx, `
INSERT INTO leave_year (
    id, employee_id, starts_on, ends_on, granted_days, extra_days, accrued_adjustment,
    unpaid_calendar_days, excess_unpaid_days, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT (employee_id, starts_on) DO UPDATE SET
    ends_on=EXCLUDED.ends_on, granted_days=EXCLUDED.granted_days, extra_days=EXCLUDED.extra_days,
    accrued_adjustment=EXCLUDED.accrued_adjustment, unpaid_calendar_days=EXCLUDED.unpaid_calendar_days,
    excess_unpaid_days=EXCLUDED.excess_unpaid_days, updated_at=EXCLUDED.updated_at
RETURNING `+yearCols,
		y.ID, y.EmployeeID, dateArg(y.StartsOn), dateArg(y.EndsOn), y.GrantedDays, y.ExtraDays, y.AccruedAdjustment,
		y.UnpaidCalendarDays, y.ExcessUnpaidDays, y.CreatedAt.UTC(), y.UpdatedAt.UTC())
	return scanYear(row)
}

func (s *pgQ) ListLeaveYears(ctx context.Context, employeeID string) ([]model.LeaveYear, error) {
	rows, err := s.q.Query(ctx, `SELECT `+yearCols+` FROM leave_year WHERE employee_id=$1 ORDER BY starts_on`, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.LeaveYear
	for rows.Next() {
		y, err := scanYear(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, y)
	}
	return out, rows.Err()
}

func (s *pgQ) LockLeaveYear(ctx context.Context, id string) error {
	var got string
	err := s.q.QueryRow(ctx, `SELECT id::text FROM leave_year WHERE id=$1 FOR UPDATE`, id).Scan(&got)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *pgQ) InsertRequest(ctx context.Context, r model.Request) error {
	_, err := s.q.Exec(ctx, `
INSERT INTO leave_request (
    id, employee_id, leave_year_id, type, status, starts_on, ends_on, counted_days, reason_code,
    certificate_number, replaces_id, successor_id, cancel_reason, overlap_choice, manager_vacant,
    self_approval, needs_confirmation, pay_by_warning, acknowledged_at, confirmed_at, processed_at,
    processed_by, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)`,
		r.ID, r.EmployeeID, uuidArg(r.LeaveYearID), r.Type, r.Status, dateArg(r.StartsOn), dateArg(r.EndsOn), r.CountedDays,
		r.ReasonCode, r.CertificateNumber, uuidArg(r.ReplacesID), uuidArg(r.SuccessorID), r.CancelReason, r.OverlapChoice,
		r.ManagerVacant, r.SelfApproval, r.NeedsConfirmation, r.PayByWarning, timePtr(r.AcknowledgedAt), timePtr(r.ConfirmedAt),
		timePtr(r.ProcessedAt), uuidArg(r.ProcessedBy), r.CreatedAt.UTC(), r.UpdatedAt.UTC())
	return err
}

func (s *pgQ) UpdateRequest(ctx context.Context, r model.Request) error {
	tag, err := s.q.Exec(ctx, `
UPDATE leave_request SET
    employee_id=$2, leave_year_id=$3, type=$4, status=$5, starts_on=$6, ends_on=$7, counted_days=$8,
    reason_code=$9, certificate_number=$10, replaces_id=$11, successor_id=$12, cancel_reason=$13,
    overlap_choice=$14, manager_vacant=$15, self_approval=$16, needs_confirmation=$17, pay_by_warning=$18,
    acknowledged_at=$19, confirmed_at=$20, processed_at=$21, processed_by=$22, updated_at=$23
WHERE id=$1`,
		r.ID, r.EmployeeID, uuidArg(r.LeaveYearID), r.Type, r.Status, dateArg(r.StartsOn), dateArg(r.EndsOn), r.CountedDays,
		r.ReasonCode, r.CertificateNumber, uuidArg(r.ReplacesID), uuidArg(r.SuccessorID), r.CancelReason, r.OverlapChoice,
		r.ManagerVacant, r.SelfApproval, r.NeedsConfirmation, r.PayByWarning, timePtr(r.AcknowledgedAt), timePtr(r.ConfirmedAt),
		timePtr(r.ProcessedAt), uuidArg(r.ProcessedBy), r.UpdatedAt.UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgQ) GetRequest(ctx context.Context, id string) (model.Request, error) {
	return scanRequest(s.q.QueryRow(ctx, `SELECT `+requestCols+` FROM leave_request WHERE id=$1`, id))
}

func (s *pgQ) ListRequests(ctx context.Context, f model.RequestFilter) ([]model.Request, error) {
	q := `SELECT ` + requestColsAliased + ` FROM leave_request r JOIN employee e ON e.id = r.employee_id WHERE 1=1`
	var args []any
	add := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if f.EmployeeID != "" {
		q += ` AND r.employee_id=` + add(f.EmployeeID)
	}
	if f.ManagerID != "" {
		q += ` AND e.manager_id=` + add(f.ManagerID)
	}
	if f.DepartmentID != "" {
		q += ` AND e.department_id=` + add(f.DepartmentID)
	}
	if len(f.Statuses) > 0 {
		q += ` AND r.status = ANY(` + add(f.Statuses) + `)`
	}
	if f.EndsOnOrAfter != nil {
		q += ` AND r.ends_on >= ` + add(dateArg(*f.EndsOnOrAfter))
	}
	if f.OverlapFrom != nil && f.OverlapTo != nil {
		q += ` AND r.starts_on <= ` + add(dateArg(*f.OverlapTo)) + ` AND r.ends_on >= ` + add(dateArg(*f.OverlapFrom))
	}
	q += ` ORDER BY r.starts_on, r.id`
	if f.Limit > 0 {
		q += ` LIMIT ` + add(f.Limit)
	}
	rows, err := s.q.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Request
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *pgQ) InsertStep(ctx context.Context, st model.Step) error {
	_, err := s.q.Exec(ctx, `
INSERT INTO approval_step (id, request_id, sequence, role, expected_employee_id, decision, comment, decided_at, actor_id)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		st.ID, st.RequestID, st.Sequence, st.Role, uuidArg(st.ExpectedEmployeeID), st.Decision, st.Comment, timePtr(st.DecidedAt), uuidArg(st.ActorID))
	return err
}

func (s *pgQ) UpdateStep(ctx context.Context, st model.Step) error {
	tag, err := s.q.Exec(ctx, `
UPDATE approval_step SET expected_employee_id=$2, decision=$3, comment=$4, decided_at=$5, actor_id=$6
WHERE id=$1`, st.ID, uuidArg(st.ExpectedEmployeeID), st.Decision, st.Comment, timePtr(st.DecidedAt), uuidArg(st.ActorID))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgQ) ListSteps(ctx context.Context, requestID string) ([]model.Step, error) {
	rows, err := s.q.Query(ctx, `
SELECT id::text, request_id::text, sequence, role, expected_employee_id::text, decision, comment, decided_at, actor_id::text
FROM approval_step WHERE request_id=$1 ORDER BY sequence`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Step
	for rows.Next() {
		st, err := scanStep(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *pgQ) UpsertCalendarDay(ctx context.Context, d model.CalendarDay) error {
	_, err := s.q.Exec(ctx, `
INSERT INTO calendar_day (day, kind, name, source, updated_at) VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (day) DO UPDATE SET kind=EXCLUDED.kind, name=EXCLUDED.name, source=EXCLUDED.source, updated_at=EXCLUDED.updated_at`,
		dateArg(d.Date), d.Kind, d.Name, d.Source, d.UpdatedAt.UTC())
	return err
}

func (s *pgQ) ListCalendarDays(ctx context.Context, from, to time.Time) ([]model.CalendarDay, error) {
	rows, err := s.q.Query(ctx, `
SELECT to_char(day, 'YYYY-MM-DD'), kind, name, source, updated_at
FROM calendar_day WHERE day >= $1 AND day <= $2 ORDER BY day`, dateArg(from), dateArg(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.CalendarDay
	for rows.Next() {
		var ds string
		var d model.CalendarDay
		if err := rows.Scan(&ds, &d.Kind, &d.Name, &d.Source, &d.UpdatedAt); err != nil {
			return nil, err
		}
		parsed, err := time.Parse("2006-01-02", ds)
		if err != nil {
			return nil, err
		}
		d.Date = parsed.UTC()
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *pgQ) InsertNotification(ctx context.Context, n model.Notification) error {
	_, err := s.q.Exec(ctx, `
INSERT INTO notification (
    id, request_id, recipient_employee_id, kind, recipient_email, language, due_at, sent_at, last_error, notice_created, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT (request_id, kind, recipient_employee_id) DO NOTHING`,
		n.ID, n.RequestID, n.RecipientEmployeeID, n.Kind, n.RecipientEmail, emptyLocale(n.Language), n.DueAt.UTC(), timePtr(n.SentAt), n.LastError, n.NoticeCreated, n.CreatedAt.UTC())
	return err
}

func (s *pgQ) ListUnsentDue(ctx context.Context, now time.Time) ([]model.Notification, error) {
	rows, err := s.q.Query(ctx, `SELECT `+noteCols+` FROM notification WHERE sent_at IS NULL AND due_at <= $1 ORDER BY due_at, id`, now.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNotes(rows)
}

func (s *pgQ) UpdateNotification(ctx context.Context, n model.Notification) error {
	tag, err := s.q.Exec(ctx, `
UPDATE notification SET recipient_email=$2, language=$3, due_at=$4, sent_at=$5, last_error=$6, notice_created=$7
WHERE id=$1`, n.ID, n.RecipientEmail, emptyLocale(n.Language), n.DueAt.UTC(), timePtr(n.SentAt), n.LastError, n.NoticeCreated)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgQ) InsertNotice(ctx context.Context, n model.Notice) error {
	_, err := s.q.Exec(ctx, `
INSERT INTO portal_notice (id, employee_id, request_id, notification_id, kind, title, body, created_at, read_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		n.ID, n.EmployeeID, uuidArg(n.RequestID), uuidArg(n.NotificationID), n.Kind, n.Title, n.Body, n.CreatedAt.UTC(), timePtr(n.ReadAt))
	return err
}

func (s *pgQ) ListNotices(ctx context.Context, employeeID string) ([]model.Notice, error) {
	rows, err := s.q.Query(ctx, `
SELECT id::text, employee_id::text, request_id::text, notification_id::text, kind, title, body, created_at, read_at
FROM portal_notice WHERE employee_id=$1 ORDER BY created_at DESC`, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Notice
	for rows.Next() {
		n, err := scanNotice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *pgQ) MarkNoticeRead(ctx context.Context, id, employeeID string, at time.Time) error {
	tag, err := s.q.Exec(ctx, `UPDATE portal_notice SET read_at=$3 WHERE id=$1 AND employee_id=$2`, id, employeeID, at.UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgQ) InsertAudit(ctx context.Context, a model.Audit) error {
	_, err := s.q.Exec(ctx, `
INSERT INTO audit_event (id, actor_id, request_id, employee_id, action, before_status, after_status, detail, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		a.ID, uuidArg(a.ActorID), uuidArg(a.RequestID), uuidArg(a.EmployeeID), a.Action, a.BeforeStatus, a.AfterStatus, jsonArg(a.Detail), a.CreatedAt.UTC())
	return err
}

func (s *pgQ) UpsertScheduleLock(ctx context.Context, lock model.ScheduleLock) error {
	_, err := s.q.Exec(ctx, `
INSERT INTO schedule_lock (year, locked_by, locked_at) VALUES ($1,$2,$3)
ON CONFLICT (year) DO UPDATE SET locked_by=EXCLUDED.locked_by, locked_at=EXCLUDED.locked_at`,
		lock.Year, uuidArg(lock.LockedBy), lock.LockedAt.UTC())
	return err
}

func (s *pgQ) GetScheduleLock(ctx context.Context, year int) (model.ScheduleLock, error) {
	var lock model.ScheduleLock
	var by *string
	err := s.q.QueryRow(ctx, `SELECT year, locked_by::text, locked_at FROM schedule_lock WHERE year=$1`, year).Scan(&lock.Year, &by, &lock.LockedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ScheduleLock{}, ErrNotFound
	}
	if err != nil {
		return model.ScheduleLock{}, err
	}
	if by != nil {
		lock.LockedBy = *by
	}
	return lock, nil
}

const employeeCols = `id::text, keycloak_sub, display_name, email, locale, locale_chosen, employee_number, position, department_id::text, manager_id::text, manager_raw, to_char(hire_date, 'YYYY-MM-DD'), active, hr, extra_days, early_leave, raw_attributes, last_sync_at, created_at, updated_at`

const yearCols = `id::text, employee_id::text, to_char(starts_on, 'YYYY-MM-DD'), to_char(ends_on, 'YYYY-MM-DD'), granted_days, extra_days, accrued_adjustment, unpaid_calendar_days, excess_unpaid_days, created_at, updated_at`

const requestCols = `id::text, employee_id::text, leave_year_id::text, type, status, to_char(starts_on, 'YYYY-MM-DD'), to_char(ends_on, 'YYYY-MM-DD'), counted_days, reason_code, certificate_number, replaces_id::text, successor_id::text, cancel_reason, overlap_choice, manager_vacant, self_approval, needs_confirmation, pay_by_warning, acknowledged_at, confirmed_at, processed_at, processed_by::text, created_at, updated_at`

const requestColsAliased = `r.id::text, r.employee_id::text, r.leave_year_id::text, r.type, r.status, to_char(r.starts_on, 'YYYY-MM-DD'), to_char(r.ends_on, 'YYYY-MM-DD'), r.counted_days, r.reason_code, r.certificate_number, r.replaces_id::text, r.successor_id::text, r.cancel_reason, r.overlap_choice, r.manager_vacant, r.self_approval, r.needs_confirmation, r.pay_by_warning, r.acknowledged_at, r.confirmed_at, r.processed_at, r.processed_by::text, r.created_at, r.updated_at`

const noteCols = `id::text, request_id::text, recipient_employee_id::text, kind, recipient_email, language, due_at, sent_at, last_error, notice_created, created_at`

type scanner interface{ Scan(dest ...any) error }

func scanEmployee(row scanner) (model.Employee, error) {
	var e model.Employee
	var dept, manager, hire *string
	var raw []byte
	err := row.Scan(&e.ID, &e.Subject, &e.DisplayName, &e.Email, &e.Locale, &e.LocaleChosen, &e.EmployeeNumber, &e.Position, &dept, &manager, &e.ManagerRaw, &hire, &e.Active, &e.HR, &e.ExtraDays, &e.EarlyLeave, &raw, &e.LastSyncAt, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Employee{}, ErrNotFound
	}
	if err != nil {
		return model.Employee{}, err
	}
	e.DepartmentID = deref(dept)
	e.ManagerID = deref(manager)
	e.HireDate = parseDatePtr(hire)
	e.Raw = raw
	return e, nil
}

func scanUnit(row scanner) (model.OrgUnit, error) {
	var u model.OrgUnit
	var parent *string
	err := row.Scan(&u.ID, &u.ExternalID, &u.Name, &parent, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.OrgUnit{}, ErrNotFound
	}
	if err != nil {
		return model.OrgUnit{}, err
	}
	u.ParentID = deref(parent)
	return u, nil
}

func scanYear(row scanner) (model.LeaveYear, error) {
	var y model.LeaveYear
	var start, end string
	err := row.Scan(&y.ID, &y.EmployeeID, &start, &end, &y.GrantedDays, &y.ExtraDays, &y.AccruedAdjustment, &y.UnpaidCalendarDays, &y.ExcessUnpaidDays, &y.CreatedAt, &y.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.LeaveYear{}, ErrNotFound
	}
	if err != nil {
		return model.LeaveYear{}, err
	}
	y.StartsOn, err = time.Parse("2006-01-02", start)
	if err != nil {
		return model.LeaveYear{}, err
	}
	y.EndsOn, err = time.Parse("2006-01-02", end)
	if err != nil {
		return model.LeaveYear{}, err
	}
	y.StartsOn, y.EndsOn = y.StartsOn.UTC(), y.EndsOn.UTC()
	return y, nil
}

func scanRequest(row scanner) (model.Request, error) {
	var r model.Request
	var start, end string
	var yearID, replaces, successor, processedBy *string
	err := row.Scan(&r.ID, &r.EmployeeID, &yearID, &r.Type, &r.Status, &start, &end, &r.CountedDays, &r.ReasonCode, &r.CertificateNumber, &replaces, &successor, &r.CancelReason, &r.OverlapChoice, &r.ManagerVacant, &r.SelfApproval, &r.NeedsConfirmation, &r.PayByWarning, &r.AcknowledgedAt, &r.ConfirmedAt, &r.ProcessedAt, &processedBy, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Request{}, ErrNotFound
	}
	if err != nil {
		return model.Request{}, err
	}
	r.LeaveYearID = deref(yearID)
	r.ReplacesID = deref(replaces)
	r.SuccessorID = deref(successor)
	r.ProcessedBy = deref(processedBy)
	r.StartsOn, err = time.Parse("2006-01-02", start)
	if err != nil {
		return model.Request{}, err
	}
	r.EndsOn, err = time.Parse("2006-01-02", end)
	if err != nil {
		return model.Request{}, err
	}
	r.StartsOn, r.EndsOn = r.StartsOn.UTC(), r.EndsOn.UTC()
	return r, nil
}

func scanStep(row scanner) (model.Step, error) {
	var st model.Step
	var expected, actor *string
	err := row.Scan(&st.ID, &st.RequestID, &st.Sequence, &st.Role, &expected, &st.Decision, &st.Comment, &st.DecidedAt, &actor)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Step{}, ErrNotFound
	}
	if err != nil {
		return model.Step{}, err
	}
	st.ExpectedEmployeeID = deref(expected)
	st.ActorID = deref(actor)
	return st, nil
}

func scanNotes(rows pgx.Rows) ([]model.Notification, error) {
	var out []model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ID, &n.RequestID, &n.RecipientEmployeeID, &n.Kind, &n.RecipientEmail, &n.Language, &n.DueAt, &n.SentAt, &n.LastError, &n.NoticeCreated, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func scanNotice(row scanner) (model.Notice, error) {
	var n model.Notice
	var req, notif *string
	err := row.Scan(&n.ID, &n.EmployeeID, &req, &notif, &n.Kind, &n.Title, &n.Body, &n.CreatedAt, &n.ReadAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Notice{}, ErrNotFound
	}
	if err != nil {
		return model.Notice{}, err
	}
	n.RequestID = deref(req)
	n.NotificationID = deref(notif)
	return n, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func parseDatePtr(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func dateArg(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format("2006-01-02")
}

func datePtr(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.UTC().Format("2006-01-02")
}

func timePtr(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.UTC()
}

func uuidArg(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func jsonArg(b []byte) []byte {
	if len(b) == 0 {
		return []byte(`{}`)
	}
	return b
}

func emptyLocale(s string) string {
	if s == "" {
		return "ru"
	}
	return s
}
