package notify

import (
	"bytes"
	"html/template"
	"strings"
	"sync"

	_ "embed"
)

//go:embed templates/mail.html
var mailSource string

var (
	mailOnce sync.Once
	mailTpl  *template.Template
	mailErr  error
)

// Vars is the data for one notice. Type is vacation, day_off, or sick.
// Role is employee, manager, or hr. An empty role is chosen from Kind.
type Vars struct {
	Kind           string
	Locale         string
	Employee       string
	Start          string
	End            string
	Type           string
	Role           string
	Moved          bool
	CountedDays    int
	PayBy          string
	EmployeeNumber string
}

type mailData struct {
	Lang    string
	Eyebrow string
	Title   string
	Lead    string
	Detail  string
	Chip    string
	Footer  string
	Product string
	Tone    string
	Texture string
}

const mailTexture = "" +
	"        ..\n" +
	"       .##.\n" +
	"      .#  #.\n" +
	"       .##.\n" +
	"      .#  #.\n" +
	"     .#    #.\n" +
	"    .#      #.\n" +
	"     .#    #.\n" +
	"      .####.\n" +
	"     .#    #.\n" +
	"    .#      #."

// Render builds the plain text and the HTML for one notice.
func Render(v Vars) Content {
	text := composeText(v)
	html, err := renderHTML(v, text)
	if err != nil {
		text.HTML = ""
		return text
	}
	text.HTML = html
	return text
}

func renderHTML(v Vars, text Content) (string, error) {
	mailOnce.Do(func() {
		mailTpl, mailErr = template.New("mail").Parse(mailSource)
	})
	if mailErr != nil {
		return "", mailErr
	}
	ru := v.Locale != "en"
	data := mailData{
		Lang:    "ru",
		Eyebrow: eyebrow(v.role(), ru),
		Title:   text.Title,
		Lead:    text.Body,
		Detail:  detail(v, ru),
		Chip:    strings.TrimSpace(v.Start + " — " + v.End),
		Footer:  "Это письмо сервиса отпусков. Отвечать на него не нужно.",
		Product: "Отпуска",
		Tone:    "go",
		Texture: mailTexture,
	}
	if !ru {
		data.Lang = "en"
		data.Footer = "This message is from the leave service. There is no need to reply."
		data.Product = "Leave"
	}
	if v.Kind == KindDeclined {
		data.Tone = "stop"
	}
	var buf bytes.Buffer
	if err := mailTpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (v Vars) role() string {
	if v.Role != "" {
		return v.Role
	}
	switch v.Kind {
	case KindApprovalManager, KindRemind3Manager:
		return "manager"
	case KindApprovalHR, KindRemind3HR, KindConfirmedHR, KindApprovedHR:
		return "hr"
	case KindFinance:
		return "finance"
	default:
		return "employee"
	}
}

func eyebrow(role string, ru bool) string {
	switch role {
	case "manager":
		if ru {
			return "Руководителю"
		}
		return "Manager"
	case "hr":
		if ru {
			return "Кадрам"
		}
		return "HR"
	case "finance":
		if ru {
			return "Финансам"
		}
		return "Finance"
	default:
		if ru {
			return "Сотруднику"
		}
		return "Employee"
	}
}

func detail(v Vars, ru bool) string {
	if !v.Moved {
		return ""
	}
	if ru {
		return "Прежние даты остаются, пока кадры не согласуют новые."
	}
	return "The previous dates stay until HR approves the new ones."
}

func typeLabel(typ string, ru bool) string {
	switch typ {
	case "day_off":
		if ru {
			return "отгул"
		}
		return "day off"
	case "sick":
		if ru {
			return "больничный"
		}
		return "sick leave"
	default:
		if ru {
			return "отпуск"
		}
		return "vacation"
	}
}
