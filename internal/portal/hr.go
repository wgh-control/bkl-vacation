package portal

import (
	"context"
	"strconv"
	"time"

	"github.com/wgh-control/bkl-vacation/internal/calendar"
	"github.com/wgh-control/bkl-vacation/internal/identity"
	"github.com/wgh-control/bkl-vacation/internal/leave"
	"github.com/wgh-control/bkl-vacation/internal/model"
	"github.com/wgh-control/bkl-vacation/internal/report"
	"github.com/wgh-control/bkl-vacation/internal/store"
)

// HRQueue is every request waiting for HR, including an HR person's own request
// after their manager has approved it.
func (s *Service) HRQueue(ctx context.Context, p identity.Principal) ([]QueueItem, error) {
	var out []QueueItem
	err := s.tx(ctx, func(q store.Queries) error {
		if _, err := s.requireHR(ctx, q, p); err != nil {
			return err
		}
		reqs, err := q.ListRequests(ctx, model.RequestFilter{Statuses: []string{string(model.StatusPendingHR)}})
		if err != nil {
			return err
		}
		out, err = s.queue(ctx, q, reqs, "hr")
		return err
	})
	return out, err
}

// MarkProcessed is the HR document step. It is only valid after the employee confirmed.
func (s *Service) MarkProcessed(ctx context.Context, p identity.Principal, id string) (RequestView, error) {
	var out RequestView
	err := s.tx(ctx, func(q store.Queries) error {
		actor, err := s.requireHR(ctx, q, p)
		if err != nil {
			return err
		}
		req, err := q.GetRequest(ctx, id)
		if err != nil {
			return missing(err)
		}
		if req.Type != model.TypeVacation || req.Status != model.StatusConfirmed {
			return code("invalid_status", "only a confirmed vacation can be marked processed", 409)
		}
		owner, err := q.GetEmployee(ctx, req.EmployeeID)
		if err != nil {
			return err
		}
		now := s.now()
		before := req.Status
		req.Status = model.StatusProcessed
		req.ProcessedAt = &now
		req.ProcessedBy = actor.ID
		req.UpdatedAt = now
		if err := q.UpdateRequest(ctx, req); err != nil {
			return err
		}
		if err := s.ensureRequestNotices(ctx, q, req, owner); err != nil {
			return err
		}
		if err := s.audit(ctx, q, actor.ID, req.ID, owner.ID, "processed", string(before), string(req.Status)); err != nil {
			return err
		}
		out, err = s.viewRequest(ctx, q, req, "hr")
		return err
	})
	return out, err
}

// Export downloads approved requests for one user, one department, or everyone.
// format is csv, xlsx, xls, or pdf. Header labels follow the HR user's language.
func (s *Service) Export(ctx context.Context, p identity.Principal, scope, userID, departmentID, format, from, to string) (File, error) {
	var file File
	err := s.tx(ctx, func(q store.Queries) error {
		hr, err := s.requireHR(ctx, q, p)
		if err != nil {
			return err
		}
		f := model.RequestFilter{Statuses: []string{
			string(model.StatusScheduled), string(model.StatusAwaitingConfirmation),
			string(model.StatusConfirmed), string(model.StatusProcessed),
		}}
		switch scope {
		case "user":
			if userID == "" {
				return code("export_scope", "user_id is required when scope is user", 400)
			}
			if _, err := q.GetEmployee(ctx, userID); err != nil {
				return missing(err)
			}
			f.EmployeeID = userID
		case "department":
			if departmentID == "" {
				return code("export_scope", "department_id is required when scope is department", 400)
			}
			if _, err := q.GetOrgUnit(ctx, departmentID); err != nil {
				return missing(err)
			}
			f.DepartmentID = departmentID
		case "all":
		default:
			return code("export_scope", "scope must be user, department, or all", 400)
		}
		if from != "" || to != "" {
			if from == "" || to == "" {
				return code("validation", "from and to are set together", 400)
			}
			fs, err := leave.ParseDate(from)
			if err != nil {
				return code("validation", "from must be YYYY-MM-DD", 400)
			}
			ts, err := leave.ParseDate(to)
			if err != nil {
				return code("validation", "to must be YYYY-MM-DD", 400)
			}
			f.OverlapFrom = &fs
			f.OverlapTo = &ts
		} else {
			today := leave.Date(s.now())
			f.EndsOnOrAfter = &today
		}
		reqs, err := q.ListRequests(ctx, f)
		if err != nil {
			return err
		}
		emps, err := q.ListEmployees(ctx)
		if err != nil {
			return err
		}
		byID := map[string]model.Employee{}
		for _, e := range emps {
			byID[e.ID] = e
		}
		units, err := q.ListOrgUnits(ctx)
		if err != nil {
			return err
		}
		unitName := map[string]string{}
		for _, u := range units {
			unitName[u.ID] = u.Name
		}
		locale := hr.Locale
		if locale != "en" {
			locale = "ru"
		}
		table := report.Table{Headers: exportHeaders(locale)}
		for _, r := range reqs {
			emp := byID[r.EmployeeID]
			mgr := byID[emp.ManagerID]
			table.Rows = append(table.Rows, []string{
				label(locale, "type", string(r.Type)),
				label(locale, "status", string(r.Status)),
				emp.DisplayName,
				emp.EmployeeNumber,
				unitName[emp.DepartmentID],
				emp.Position,
				mgr.DisplayName,
				leave.FormatDate(r.StartsOn),
				leave.FormatDate(r.EndsOn),
				strconv.Itoa(r.CountedDays),
				r.CertificateNumber,
				r.ReasonCode,
				stamp(r.ConfirmedAt),
				stamp(r.ProcessedAt),
			})
		}
		body, name, ctype, err := renderExport(format, table)
		if err != nil {
			return err
		}
		file = File{Name: name, ContentType: ctype, Body: body}
		return nil
	})
	return file, err
}

func renderExport(format string, table report.Table) ([]byte, string, string, error) {
	switch format {
	case "csv":
		return report.CSV(table), "bkl-leave.csv", "text/csv; charset=utf-8", nil
	case "xlsx":
		b, err := report.XLSX(table)
		return b, "bkl-leave.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", err
	case "xls":
		return report.XLS(table), "bkl-leave.xls", "application/vnd.ms-excel", nil
	case "pdf":
		b, err := report.PDF(table)
		return b, "bkl-leave.pdf", "application/pdf", err
	default:
		return nil, "", "", code("validation", "format must be csv, xlsx, xls, or pdf", 400)
	}
}

func exportHeaders(locale string) []string {
	if locale == "en" {
		return []string{"Type", "Status", "Employee", "Employee number", "Department", "Position", "Manager", "Start", "End", "Counted days", "Sick certificate", "Day-off reason", "Confirmed at", "Processed at"}
	}
	return []string{"Тип", "Статус", "Сотрудник", "Табельный номер", "Подразделение", "Должность", "Руководитель", "Начало", "Окончание", "Дней", "Номер больничного", "Причина отгула", "Подтверждён", "Оформлен"}
}

func label(locale, kind, code string) string {
	en := locale == "en"
	switch kind + ":" + code {
	case "type:vacation":
		if en {
			return "Vacation"
		}
		return "Отпуск"
	case "type:day_off":
		if en {
			return "Day off"
		}
		return "Отгул"
	case "type:sick":
		if en {
			return "Sick day"
		}
		return "Больничный"
	case "status:scheduled":
		if en {
			return "Scheduled"
		}
		return "Запланирован"
	case "status:awaiting_confirmation":
		if en {
			return "Awaiting confirmation"
		}
		return "Ожидает подтверждения"
	case "status:confirmed":
		if en {
			return "Confirmed"
		}
		return "Подтверждён"
	case "status:processed":
		if en {
			return "Processed"
		}
		return "Оформлен"
	default:
		return code
	}
}

// Warnings lists article 124 carryover and two-year gaps for HR.
func (s *Service) Warnings(ctx context.Context, p identity.Principal) ([]WarningView, error) {
	var out []WarningView
	err := s.tx(ctx, func(q store.Queries) error {
		if _, err := s.requireHR(ctx, q, p); err != nil {
			return err
		}
		emps, err := q.ListEmployees(ctx)
		if err != nil {
			return err
		}
		units, err := q.ListOrgUnits(ctx)
		if err != nil {
			return err
		}
		names := map[string]string{}
		for _, u := range units {
			names[u.ID] = u.Name
		}
		for _, emp := range emps {
			if !emp.Active || emp.HireDate == nil {
				continue
			}
			bal, err := s.balanceFor(ctx, q, emp)
			if err != nil {
				return err
			}
			for _, y := range bal.WorkingYears {
				if y.CarryoverWarning {
					out = append(out, WarningView{
						EmployeeID: emp.ID, EmployeeName: emp.DisplayName, Department: names[emp.DepartmentID],
						Code: "carryover", WorkingYear: y.StartsOn,
						Message: "unused leave is inside 60 days of the article 124 deadline or past it",
					})
				}
				if y.TwoYearWarning {
					out = append(out, WarningView{
						EmployeeID: emp.ID, EmployeeName: emp.DisplayName, Department: names[emp.DepartmentID],
						Code: "two_years", WorkingYear: y.StartsOn,
						Message: "no annual leave in two consecutive working years",
					})
				}
			}
		}
		if out == nil {
			out = []WarningView{}
		}
		return nil
	})
	return out, err
}

// Calendar returns the production calendar for the signed-in person's language.
func (s *Service) Calendar(ctx context.Context, p identity.Principal, from, to string) ([]CalendarDayView, error) {
	start, err := leave.ParseDate(from)
	if err != nil {
		return nil, code("validation", "from must be YYYY-MM-DD", 400)
	}
	end, err := leave.ParseDate(to)
	if err != nil {
		return nil, code("validation", "to must be YYYY-MM-DD", 400)
	}
	if end.Before(start) || leave.DaysInclusive(start, end) > 400 {
		return nil, code("validation", "calendar range must be within 400 days", 400)
	}
	var out []CalendarDayView
	err = s.tx(ctx, func(q store.Queries) error {
		emp, err := s.require(ctx, q, p, false)
		if err != nil {
			return err
		}
		stored, err := q.ListCalendarDays(ctx, start, end)
		if err != nil {
			return err
		}
		var ov []calendar.Day
		for _, d := range stored {
			ov = append(ov, calendar.Day{Date: d.Date, Kind: calendar.Kind(d.Kind), Name: d.Name, Source: d.Source})
		}
		locale := emp.Locale
		if locale != "en" {
			locale = "ru"
		}
		for _, d := range calendar.Build(start, end, ov, locale) {
			out = append(out, CalendarDayView{Date: leave.FormatDate(d.Date), Kind: string(d.Kind), Name: d.Name, Source: d.Source})
		}
		return nil
	})
	return out, err
}

// PutCalendarDay stores a government transfer or a regional holiday.
func (s *Service) PutCalendarDay(ctx context.Context, p identity.Principal, date, kind, name, source string) (CalendarDayView, error) {
	day, err := leave.ParseDate(date)
	if err != nil {
		return CalendarDayView{}, code("validation", "date must be YYYY-MM-DD", 400)
	}
	switch kind {
	case "work", "weekend", "holiday", "transferred":
	default:
		return CalendarDayView{}, code("validation", "kind must be work, weekend, holiday, or transferred", 400)
	}
	if source == "" {
		return CalendarDayView{}, code("validation", "source is required", 400)
	}
	err = s.tx(ctx, func(q store.Queries) error {
		if _, err := s.requireHR(ctx, q, p); err != nil {
			return err
		}
		return q.UpsertCalendarDay(ctx, model.CalendarDay{Date: day, Kind: kind, Name: name, Source: source, UpdatedAt: s.now()})
	})
	if err != nil {
		return CalendarDayView{}, err
	}
	return CalendarDayView{Date: date, Kind: kind, Name: name, Source: source}, nil
}

// SetEntitlement records extra days above 28 and an early-leave exception (D4).
func (s *Service) SetEntitlement(ctx context.Context, p identity.Principal, employeeID string, extra int, early bool) (Profile, error) {
	if extra < 0 || extra > 60 {
		return Profile{}, code("validation", "extra_days must be between 0 and 60", 400)
	}
	var out Profile
	err := s.tx(ctx, func(q store.Queries) error {
		if _, err := s.requireHR(ctx, q, p); err != nil {
			return err
		}
		emp, err := q.GetEmployee(ctx, employeeID)
		if err != nil {
			return missing(err)
		}
		emp.ExtraDays = extra
		emp.EarlyLeave = early
		emp.UpdatedAt = s.now()
		if err := q.UpdateEmployee(ctx, emp); err != nil {
			return err
		}
		years, err := q.ListLeaveYears(ctx, emp.ID)
		if err != nil {
			return err
		}
		today := leave.Date(s.now())
		for _, y := range years {
			if !y.EndsOn.Before(today) {
				y.ExtraDays = extra
				y.UpdatedAt = s.now()
				if _, err := q.UpsertLeaveYear(ctx, y); err != nil {
					return err
				}
			}
		}
		out, err = s.profile(ctx, q, emp)
		return err
	})
	return out, err
}

// LockSchedule records that HR locked the annual vacation schedule for a calendar year.
func (s *Service) LockSchedule(ctx context.Context, p identity.Principal, year int) (ScheduleView, error) {
	if year < 2000 || year > 2100 {
		return ScheduleView{}, code("validation", "year is out of range", 400)
	}
	var out ScheduleView
	err := s.tx(ctx, func(q store.Queries) error {
		actor, err := s.requireHR(ctx, q, p)
		if err != nil {
			return err
		}
		lock := model.ScheduleLock{Year: year, LockedBy: actor.ID, LockedAt: s.now()}
		if err := q.UpsertScheduleLock(ctx, lock); err != nil {
			return err
		}
		out = ScheduleView{Year: year, Locked: true, LockedBy: actor.ID, LockedAt: lock.LockedAt.UTC().Format(time.RFC3339)}
		return nil
	})
	return out, err
}

// Schedule reports whether a calendar year is locked.
func (s *Service) Schedule(ctx context.Context, p identity.Principal, year int) (ScheduleView, error) {
	var out ScheduleView
	err := s.tx(ctx, func(q store.Queries) error {
		if _, err := s.requireHR(ctx, q, p); err != nil {
			return err
		}
		lock, err := q.GetScheduleLock(ctx, year)
		if err != nil {
			if err == store.ErrNotFound {
				out = ScheduleView{Year: year, Locked: false}
				return nil
			}
			return err
		}
		out = ScheduleView{Year: year, Locked: true, LockedBy: lock.LockedBy, LockedAt: lock.LockedAt.UTC().Format(time.RFC3339)}
		return nil
	})
	return out, err
}

// ListTeam returns direct reports' requests. Sick certificate numbers stay off this list
// unless the request is waiting on this manager.
func (s *Service) ListTeam(ctx context.Context, p identity.Principal) ([]RequestView, error) {
	var out []RequestView
	err := s.tx(ctx, func(q store.Queries) error {
		emp, err := s.require(ctx, q, p, false)
		if err != nil {
			return err
		}
		reqs, err := q.ListRequests(ctx, model.RequestFilter{ManagerID: emp.ID})
		if err != nil {
			return err
		}
		out, err = s.views(ctx, q, reqs, "manager")
		return err
	})
	return out, err
}

// Notices are the in-portal copies. They exist even when SMTP has not sent the mail.
func (s *Service) Notices(ctx context.Context, p identity.Principal) ([]NoticeView, error) {
	var out []NoticeView
	err := s.tx(ctx, func(q store.Queries) error {
		emp, err := s.require(ctx, q, p, false)
		if err != nil {
			return err
		}
		rows, err := q.ListNotices(ctx, emp.ID)
		if err != nil {
			return err
		}
		for _, n := range rows {
			out = append(out, NoticeView{
				ID: n.ID, Kind: n.Kind, Title: n.Title, Body: n.Body, RequestID: n.RequestID,
				CreatedAt: n.CreatedAt.UTC().Format(time.RFC3339), ReadAt: stamp(n.ReadAt),
			})
		}
		if out == nil {
			out = []NoticeView{}
		}
		return nil
	})
	return out, err
}

// MarkNoticeRead marks one portal notice.
func (s *Service) MarkNoticeRead(ctx context.Context, p identity.Principal, id string) error {
	return s.tx(ctx, func(q store.Queries) error {
		emp, err := s.require(ctx, q, p, false)
		if err != nil {
			return err
		}
		if err := q.MarkNoticeRead(ctx, id, emp.ID, s.now()); err != nil {
			return missing(err)
		}
		return nil
	})
}

// DayOffReasons is the v1 catalog. Which reasons exist in the company is decision D5.
func DayOffReasons() []ReasonView {
	out := make([]ReasonView, 0, len(reasonCatalog))
	for _, r := range reasonCatalog {
		out = append(out, ReasonView{Code: r.Code, Unpaid: r.Unpaid, ManagerMustGrant: r.Mandatory, MaxCalendarDays: r.MaxDays})
	}
	return out
}
