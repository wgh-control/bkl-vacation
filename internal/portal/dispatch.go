package portal

import (
	"context"
	"errors"
	"time"

	"github.com/wgh-control/bkl-vacation/internal/model"
	"github.com/wgh-control/bkl-vacation/internal/notify"
	"github.com/wgh-control/bkl-vacation/internal/store"
)

type outbound struct {
	note model.Notification
	msg  notify.Message
}

// Dispatch creates due portal notices and sends mail. SMTP failures stay unsent for the next pass.
func (s *Service) Dispatch(ctx context.Context, sender notify.Sender) error {
	var batch []outbound
	err := s.tx(ctx, func(q store.Queries) error {
		if err := s.rerouteOrphans(ctx, q); err != nil {
			return err
		}
		reqs, err := q.ListRequests(ctx, model.RequestFilter{Statuses: []string{
			string(model.StatusScheduled), string(model.StatusAwaitingConfirmation),
			string(model.StatusConfirmed), string(model.StatusProcessed),
		}})
		if err != nil {
			return err
		}
		for i := range reqs {
			if err := s.refreshConfirmation(ctx, q, &reqs[i]); err != nil {
				return err
			}
			owner, err := q.GetEmployee(ctx, reqs[i].EmployeeID)
			if err != nil {
				return err
			}
			if err := s.ensureRequestNotices(ctx, q, reqs[i], owner); err != nil {
				return err
			}
		}
		due, err := q.ListUnsentDue(ctx, s.now())
		if err != nil {
			return err
		}
		for _, n := range due {
			req, err := q.GetRequest(ctx, n.RequestID)
			if err != nil {
				return err
			}
			if holdMail(n.Kind, req) {
				now := s.now()
				n.SentAt = &now
				n.LastError = "skipped"
				if err := q.UpdateNotification(ctx, n); err != nil {
					return err
				}
				continue
			}
			owner, err := q.GetEmployee(ctx, req.EmployeeID)
			if err != nil {
				return err
			}
			content := notify.Render(notify.Vars{
				Kind: n.Kind, Locale: n.Language, Employee: owner.DisplayName,
				Start: req.StartsOn.Format("2006-01-02"), End: req.EndsOn.Format("2006-01-02"),
				Type: string(req.Type), Moved: req.ReplacesID != "",
			})
			if !n.NoticeCreated {
				if err := q.InsertNotice(ctx, model.Notice{
					ID: s.id(), EmployeeID: n.RecipientEmployeeID, RequestID: req.ID, NotificationID: n.ID,
					Kind: n.Kind, Title: content.Title, Body: content.Body, CreatedAt: s.now(),
				}); err != nil {
					return err
				}
				n.NoticeCreated = true
			}
			if n.RecipientEmail == "" {
				n.LastError = "no email"
				if err := q.UpdateNotification(ctx, n); err != nil {
					return err
				}
				continue
			}
			if err := q.UpdateNotification(ctx, n); err != nil {
				return err
			}
			batch = append(batch, outbound{note: n, msg: notify.Message{To: n.RecipientEmail, Subject: content.Subject, Body: content.Body, HTML: content.HTML}})
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, item := range batch {
		var sendErr error
		if sender == nil {
			sendErr = notify.ErrNotConfigured
		} else {
			sendErr = sender.Send(ctx, item.msg)
		}
		note := item.note
		if sendErr != nil {
			note.LastError = sendErr.Error()
		} else {
			now := s.now()
			note.SentAt = &now
			note.LastError = ""
		}
		if err := s.tx(ctx, func(q store.Queries) error {
			return q.UpdateNotification(ctx, note)
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureRequestNotices(ctx context.Context, q store.Queries, req model.Request, owner model.Employee) error {
	hrs, err := s.hrPeople(ctx, q)
	if err != nil {
		return err
	}
	open := req.Status == model.StatusScheduled || req.Status == model.StatusAwaitingConfirmation || req.Status == model.StatusConfirmed || req.Status == model.StatusProcessed
	if req.NeedsConfirmation && req.AcknowledgedAt == nil && open {
		if err := s.queueNotice(ctx, q, req, owner, notify.KindNotice14); err != nil {
			return err
		}
	}
	if req.NeedsConfirmation && req.ConfirmedAt == nil && (req.Status == model.StatusScheduled || req.Status == model.StatusAwaitingConfirmation) {
		if err := s.queueNotice(ctx, q, req, owner, notify.KindConfirm5); err != nil {
			return err
		}
		if err := s.queueNotice(ctx, q, req, owner, notify.KindRemind3Employee); err != nil {
			return err
		}
		if ok, mgr := s.managerOf(ctx, q, owner); ok {
			if err := s.queueNotice(ctx, q, req, mgr, notify.KindRemind3Manager); err != nil {
				return err
			}
		}
		for _, hr := range hrs {
			if err := s.queueNotice(ctx, q, req, hr, notify.KindRemind3HR); err != nil {
				return err
			}
		}
	}
	if req.Status == model.StatusConfirmed || req.Status == model.StatusProcessed {
		for _, hr := range hrs {
			if err := s.queueNotice(ctx, q, req, hr, notify.KindConfirmedHR); err != nil {
				return err
			}
		}
	}
	if req.Status == model.StatusProcessed {
		if err := s.queueNotice(ctx, q, req, owner, notify.KindProcessedEmployee); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) queueSubmitted(ctx context.Context, q store.Queries, owner model.Employee, req model.Request) error {
	if err := s.queueNotice(ctx, q, req, owner, notify.KindSubmitted); err != nil {
		return err
	}
	if req.Status == model.StatusPendingManager {
		if ok, mgr := s.managerOf(ctx, q, owner); ok {
			return s.queueNotice(ctx, q, req, mgr, notify.KindApprovalManager)
		}
	}
	return s.queueHRApproval(ctx, q, owner, req)
}

func (s *Service) queueHRApproval(ctx context.Context, q store.Queries, owner model.Employee, req model.Request) error {
	hrs, err := s.hrPeople(ctx, q)
	if err != nil {
		return err
	}
	for _, hr := range hrs {
		if hr.ID == owner.ID {
			continue
		}
		if err := s.queueNotice(ctx, q, req, hr, notify.KindApprovalHR); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) queueNotice(ctx context.Context, q store.Queries, req model.Request, to model.Employee, kind string) error {
	if to.ID == "" {
		return nil
	}
	lang := to.Locale
	if lang != "en" {
		lang = "ru"
	}
	due := notify.DueInstant(kind, req.StartsOn, s.now())
	if kind == notify.KindConfirmedHR || kind == notify.KindProcessedEmployee {
		due = s.now()
	}
	return q.InsertNotification(ctx, model.Notification{
		ID: s.id(), RequestID: req.ID, RecipientEmployeeID: to.ID, Kind: kind,
		RecipientEmail: to.Email, Language: lang, DueAt: due, CreatedAt: s.now(),
	})
}

func (s *Service) hrPeople(ctx context.Context, q store.Queries) ([]model.Employee, error) {
	all, err := q.ListEmployees(ctx)
	if err != nil {
		return nil, err
	}
	var out []model.Employee
	for _, e := range all {
		if e.Active && e.HR {
			out = append(out, e)
		}
	}
	return out, nil
}

func (s *Service) rerouteOrphans(ctx context.Context, q store.Queries) error {
	reqs, err := q.ListRequests(ctx, model.RequestFilter{Statuses: []string{string(model.StatusPendingManager)}})
	if err != nil {
		return err
	}
	for _, req := range reqs {
		owner, err := q.GetEmployee(ctx, req.EmployeeID)
		if err != nil {
			return err
		}
		if ok, _ := s.managerOf(ctx, q, owner); ok {
			continue
		}
		before := req.Status
		req.Status = model.StatusPendingHR
		req.SelfApproval = owner.ManagerID != "" && owner.ManagerID == owner.ID
		req.ManagerVacant = !req.SelfApproval
		req.UpdatedAt = s.now()
		if err := q.UpdateRequest(ctx, req); err != nil {
			return err
		}
		if err := s.audit(ctx, q, "", req.ID, owner.ID, "manager_unavailable", string(before), string(req.Status)); err != nil {
			return err
		}
	}
	return nil
}

func closed(st model.Status) bool {
	return st == model.StatusRejected || st == model.StatusCancelled
}

// holdMail drops a scheduled reminder after the request is closed.
// A decline is itself the mail for a rejected request, so it still goes out.
func holdMail(kind string, req model.Request) bool {
	if closed(req.Status) && kind != notify.KindDeclined {
		return true
	}
	if kind == notify.KindApprovalManager && req.Status != model.StatusPendingManager {
		return true
	}
	if kind == notify.KindApprovalHR && req.Status != model.StatusPendingHR {
		return true
	}
	return skipReminder(kind, req)
}

func skipReminder(kind string, req model.Request) bool {
	switch kind {
	case notify.KindConfirm5, notify.KindRemind3Employee, notify.KindRemind3Manager, notify.KindRemind3HR:
		return req.ConfirmedAt != nil || req.Status == model.StatusConfirmed || req.Status == model.StatusProcessed
	default:
		return false
	}
}

// ApplyDirectory upserts people from Keycloak. It does not delete anyone.
func (s *Service) ApplyDirectory(ctx context.Context, users []DirectoryUser) error {
	return s.tx(ctx, func(q store.Queries) error {
		now := s.now()
		type saved struct {
			emp model.Employee
			raw string
		}
		var rows []saved
		for _, u := range users {
			if u.Subject == "" {
				continue
			}
			existing, err := q.GetEmployeeBySubject(ctx, u.Subject)
			found := err == nil
			if err != nil && !errors.Is(err, store.ErrNotFound) {
				return err
			}
			emp := existing
			if !found {
				emp = model.Employee{ID: s.id(), Subject: u.Subject, Locale: "ru", CreatedAt: now, Raw: []byte(`{}`)}
			}
			if u.DisplayName != "" {
				emp.DisplayName = u.DisplayName
			}
			if emp.DisplayName == "" {
				emp.DisplayName = u.Subject
			}
			if u.Email != "" {
				emp.Email = u.Email
			}
			if u.EmployeeNumber != "" {
				emp.EmployeeNumber = u.EmployeeNumber
			}
			if u.Position != "" {
				emp.Position = u.Position
			}
			if u.HireDate != nil {
				emp.HireDate = u.HireDate
			}
			if !emp.LocaleChosen && (u.Locale == "ru" || u.Locale == "en") {
				emp.Locale = u.Locale
			}
			emp.Active = u.Active
			emp.HR = u.HR
			if len(u.Raw) > 0 {
				emp.Raw = u.Raw
			}
			emp.LastSyncAt = &now
			emp.UpdatedAt = now
			if u.Department != "" {
				unit, err := s.ensureDepartment(ctx, q, u.Department, now)
				if err != nil {
					return err
				}
				emp.DepartmentID = unit.ID
			}
			if u.ManagerRaw != "" {
				emp.ManagerRaw = u.ManagerRaw
			}
			if !found {
				stored, err := q.UpsertEmployee(ctx, emp)
				if err != nil {
					return err
				}
				emp = stored
			} else if err := q.UpdateEmployee(ctx, emp); err != nil {
				return err
			}
			rows = append(rows, saved{emp: emp, raw: u.ManagerRaw})
		}
		for _, row := range rows {
			if row.raw == "" {
				continue
			}
			mgr, err := s.resolveManager(ctx, q, row.raw)
			if err != nil || mgr.ID == row.emp.ID {
				continue
			}
			row.emp.ManagerID = mgr.ID
			row.emp.ManagerRaw = row.raw
			row.emp.UpdatedAt = s.now()
			if err := q.UpdateEmployee(ctx, row.emp); err != nil {
				return err
			}
		}
		return nil
	})
}

// DirectoryUser is one account from the Keycloak sync.
type DirectoryUser struct {
	Subject        string
	DisplayName    string
	Email          string
	Locale         string
	EmployeeNumber string
	Position       string
	Department     string
	ManagerRaw     string
	HireDate       *time.Time
	Active         bool
	HR             bool
	Raw            []byte
}
