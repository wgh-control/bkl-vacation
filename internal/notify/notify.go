// Package notify builds the Russian and English messages the worker sends
// through the company's existing SMTP server.
package notify

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	KindSubmitted         = "submitted"
	KindApprovalManager   = "approval_needed_manager"
	KindApprovalHR        = "approval_needed_hr"
	KindApproved          = "approved"
	KindApprovedHR        = "approved_hr"
	KindFinance           = "finance_payment"
	KindDeclined          = "declined"
	KindNotice14          = "notice_14"
	KindConfirm5          = "confirm_5"
	KindRemind3Employee   = "remind_3_employee"
	KindRemind3Manager    = "remind_3_manager"
	KindRemind3HR         = "remind_3_hr"
	KindConfirmedHR       = "confirmed_hr"
	KindProcessedEmployee = "processed_employee"
)

// Message is one mail. HTML is the Leon template; Body stays the plain part.
// The portal notice is created even when this cannot be sent.
type Message struct {
	From    string
	To      string
	Subject string
	Body    string
	HTML    string
}

// Sender delivers one message. A nil or unconfigured sender must return an error
// so the notification stays unsent and is retried.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// ErrNotConfigured means SMTP settings were not supplied.
var ErrNotConfigured = fmt.Errorf("smtp not configured")

// Content is the subject, the portal text, and the HTML mail.
type Content struct {
	Subject string
	Title   string
	Body    string
	HTML    string
}

// Compose renders a vacation notice. Dates are civil days.
func Compose(kind, locale, employeeName, start, end string) Content {
	return Render(Vars{
		Kind: kind, Locale: locale, Employee: employeeName, Start: start, End: end, Type: "vacation",
	})
}

func composeText(v Vars) Content {
	ru := v.Locale != "en"
	kind := v.Kind
	employeeName := v.Employee
	start, end := v.Start, v.End
	label := typeLabel(v.Type, ru)
	switch kind {
	case KindSubmitted:
		if ru {
			return Content{
				Subject: "Заявка отправлена",
				Title:   "Заявка отправлена",
				Body:    fmt.Sprintf("Заявка на %s с %s по %s отправлена и ждёт согласования.", label, start, end),
			}
		}
		return Content{
			Subject: "Request submitted",
			Title:   "Request submitted",
			Body:    fmt.Sprintf("Your %s request from %s to %s has been submitted and is waiting for approval.", label, start, end),
		}
	case KindApprovalManager, KindApprovalHR:
		if ru {
			return Content{
				Subject: "Нужно согласование: " + employeeName,
				Title:   "Нужно согласование",
				Body:    fmt.Sprintf("%s: заявка на %s с %s по %s ждёт вашего согласования.", employeeName, label, start, end),
			}
		}
		return Content{
			Subject: "Approval needed: " + employeeName,
			Title:   "Approval needed",
			Body:    fmt.Sprintf("%s is waiting for your approval of %s from %s to %s.", employeeName, label, start, end),
		}
	case KindApproved:
		if ru {
			return Content{
				Subject: "Заявка согласована",
				Title:   "Заявка согласована",
				Body:    fmt.Sprintf("Заявку на %s с %s по %s согласовали.", label, start, end),
			}
		}
		return Content{
			Subject: "Request approved",
			Title:   "Request approved",
			Body:    fmt.Sprintf("Your %s request from %s to %s was approved.", label, start, end),
		}
	case KindApprovedHR:
		if ru {
			return Content{
				Subject: "Согласован отпуск: " + employeeName,
				Title:   "Отпуск согласован",
				Body:    fmt.Sprintf("%s: отпуск с %s по %s, %d дн. Его можно отправить в финансы для оплаты.%s", employeeName, start, end, v.CountedDays, payBySentence(ru, v.PayBy)),
			}
		}
		return Content{
			Subject: "Vacation approved: " + employeeName,
			Title:   "Vacation approved",
			Body:    fmt.Sprintf("%s: vacation from %s to %s, %d days. You can send it to finance for payment.%s", employeeName, start, end, v.CountedDays, payBySentence(ru, v.PayBy)),
		}
	case KindFinance:
		who := employeeName
		if v.EmployeeNumber != "" {
			if ru {
				who += ", табельный номер " + v.EmployeeNumber
			} else {
				who += ", employee number " + v.EmployeeNumber
			}
		}
		if ru {
			return Content{
				Subject: "Отпуск к оплате: " + employeeName,
				Title:   "Отпуск к оплате",
				Body:    fmt.Sprintf("%s. Отпуск с %s по %s, %d дн.%s Сумму считает бухгалтерия, это письмо её не содержит.", who, start, end, v.CountedDays, payBySentence(ru, v.PayBy)),
			}
		}
		return Content{
			Subject: "Vacation to pay: " + employeeName,
			Title:   "Vacation to pay",
			Body:    fmt.Sprintf("%s. Vacation from %s to %s, %d days.%s Payroll calculates the amount. This message does not.", who, start, end, v.CountedDays, payBySentence(ru, v.PayBy)),
		}
	case KindDeclined:
		if ru {
			return Content{
				Subject: "Заявка отклонена",
				Title:   "Заявка отклонена",
				Body:    fmt.Sprintf("Заявку на %s с %s по %s отклонили. Эти даты в график не встали.", label, start, end),
			}
		}
		return Content{
			Subject: "Request declined",
			Title:   "Request declined",
			Body:    fmt.Sprintf("Your %s request from %s to %s was declined. Those dates are not on the schedule.", label, start, end),
		}
	case KindNotice14:
		if ru {
			return Content{
				Subject: "Отпуск: дата начала " + start,
				Title:   "Дата начала отпуска",
				Body:    fmt.Sprintf("Ваш отпуск начнётся %s и закончится %s. Это уведомление не позднее чем за две недели до начала. Подтвердите в портале, что вы его видели.", start, end),
			}
		}
		return Content{
			Subject: "Vacation starts " + start,
			Title:   "Vacation start date",
			Body:    fmt.Sprintf("Your vacation starts on %s and ends on %s. This notice is the two-week start-date notice. Please acknowledge it in the portal.", start, end),
		}
	case KindConfirm5:
		if ru {
			return Content{
				Subject: "Подтвердите отпуск с " + start,
				Title:   "Подтверждение отпуска",
				Body:    fmt.Sprintf("Подтвердите отпуск с %s по %s. Если не подтвердить, отпуск останется в графике.", start, end),
			}
		}
		return Content{
			Subject: "Confirm vacation starting " + start,
			Title:   "Confirm your vacation",
			Body:    fmt.Sprintf("Please confirm the vacation from %s to %s. If you do not confirm, it stays on the schedule.", start, end),
		}
	case KindRemind3Employee:
		if ru {
			return Content{
				Subject: "Напоминание: отпуск с " + start,
				Title:   "Отпуск не подтверждён",
				Body:    fmt.Sprintf("Вы ещё не подтвердили отпуск с %s по %s. Он остаётся в графике и не отменяется автоматически.", start, end),
			}
		}
		return Content{
			Subject: "Reminder: vacation starting " + start,
			Title:   "Vacation still unconfirmed",
			Body:    fmt.Sprintf("You have not confirmed the vacation from %s to %s. It stays on the schedule and is not cancelled.", start, end),
		}
	case KindRemind3Manager, KindRemind3HR:
		if ru {
			return Content{
				Subject: "Отпуск не подтверждён: " + employeeName,
				Title:   "Сотрудник не подтвердил отпуск",
				Body:    fmt.Sprintf("%s не подтвердил отпуск с %s по %s. Отпуск остаётся в графике.", employeeName, start, end),
			}
		}
		return Content{
			Subject: "Unconfirmed vacation: " + employeeName,
			Title:   "Vacation was not confirmed",
			Body:    fmt.Sprintf("%s has not confirmed the vacation from %s to %s. It stays on the schedule.", employeeName, start, end),
		}
	case KindConfirmedHR:
		if ru {
			return Content{
				Subject: "Можно оформлять отпуск: " + employeeName,
				Title:   "Оформите отпуск",
				Body:    fmt.Sprintf("%s подтвердил отпуск с %s по %s. Можно готовить документы.", employeeName, start, end),
			}
		}
		return Content{
			Subject: "Process vacation: " + employeeName,
			Title:   "Process the vacation",
			Body:    fmt.Sprintf("%s confirmed the vacation from %s to %s. Documents can be prepared.", employeeName, start, end),
		}
	case KindProcessedEmployee:
		if ru {
			return Content{
				Subject: "Отпуск оформлен",
				Title:   "Отпуск оформлен",
				Body:    fmt.Sprintf("Отпуск с %s по %s отмечен как оформленный.", start, end),
			}
		}
		return Content{
			Subject: "Vacation processed",
			Title:   "Vacation processed",
			Body:    fmt.Sprintf("Your vacation from %s to %s has been processed.", start, end),
		}
	default:
		return Content{Subject: kind, Title: kind, Body: strings.TrimSpace(employeeName + " " + start + " " + end)}
	}
}

func payBySentence(ru bool, payBy string) string {
	if strings.TrimSpace(payBy) == "" {
		return ""
	}
	if ru {
		return " Выплатить не позднее " + payBy + "."
	}
	return " Pay no later than " + payBy + "."
}

// DueInstant returns when a scheduled notice should first be attempted.
// A due date already in the past is due now, so a late approval still notifies.
func DueInstant(kind string, start, now time.Time) time.Time {
	var due time.Time
	switch kind {
	case KindNotice14:
		due = time.Date(start.Year(), start.Month(), start.Day(), 8, 0, 0, 0, time.UTC).AddDate(0, 0, -14)
	case KindConfirm5:
		due = time.Date(start.Year(), start.Month(), start.Day(), 8, 0, 0, 0, time.UTC).AddDate(0, 0, -5)
	case KindRemind3Employee, KindRemind3Manager, KindRemind3HR:
		due = time.Date(start.Year(), start.Month(), start.Day(), 8, 0, 0, 0, time.UTC).AddDate(0, 0, -3)
	default:
		due = now
	}
	if due.Before(now) {
		return now
	}
	return due
}
