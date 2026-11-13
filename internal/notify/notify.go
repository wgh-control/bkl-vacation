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
	KindNotice14          = "notice_14"
	KindConfirm5          = "confirm_5"
	KindRemind3Employee   = "remind_3_employee"
	KindRemind3Manager    = "remind_3_manager"
	KindRemind3HR         = "remind_3_hr"
	KindConfirmedHR       = "confirmed_hr"
	KindProcessedEmployee = "processed_employee"
)

// Message is one mail. The portal notice is created even when this cannot be sent.
type Message struct {
	From    string
	To      string
	Subject string
	Body    string
}

// Sender delivers one message. A nil or unconfigured sender must return an error
// so the notification stays unsent and is retried.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// ErrNotConfigured means SMTP settings were not supplied.
var ErrNotConfigured = fmt.Errorf("smtp not configured")

// Content is the subject and body in the recipient's language.
type Content struct {
	Subject string
	Title   string
	Body    string
}

// Compose renders a notice. Dates are civil days.
func Compose(kind, locale, employeeName, start, end string) Content {
	ru := locale != "en"
	switch kind {
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
				Title:   "Отпуск подтверждён",
				Body:    fmt.Sprintf("%s подтвердил отпуск с %s по %s. Можно готовить документы.", employeeName, start, end),
			}
		}
		return Content{
			Subject: "Vacation confirmed: " + employeeName,
			Title:   "Vacation confirmed",
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
