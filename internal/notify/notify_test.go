package notify

import "testing"

func TestComposeLanguages(t *testing.T) {
	ru := Compose(KindNotice14, "ru", "Ада", "2027-06-01", "2027-06-14")
	en := Compose(KindNotice14, "en", "Ada", "2027-06-01", "2027-06-14")
	if ru.Subject == en.Subject || ru.Body == "" || en.Body == "" {
		t.Fatalf("ru %q en %q", ru.Subject, en.Subject)
	}
	if !contains(ru.Body, "две недели") {
		t.Fatal(ru.Body)
	}
	remind := Compose(KindRemind3HR, "en", "Ada", "2027-06-01", "2027-06-14")
	if !contains(remind.Body, "stays on the schedule") {
		t.Fatal(remind.Body)
	}
}

func TestFormatMessageUTF8(t *testing.T) {
	raw := formatMessage(Message{From: "hr@example.com", To: "a@example.com", Subject: "Отпуск", Body: "Текст"})
	if !contains(raw, "charset=UTF-8") || !contains(raw, "Текст") {
		t.Fatal(raw)
	}
}

func TestHTMLTemplates(t *testing.T) {
	for _, kind := range []string{
		KindSubmitted, KindApprovalManager, KindApprovalHR, KindApproved, KindDeclined,
		KindNotice14, KindConfirm5, KindRemind3Employee, KindRemind3Manager, KindRemind3HR,
		KindConfirmedHR, KindProcessedEmployee,
	} {
		for _, locale := range []string{"ru", "en"} {
			got := Compose(kind, locale, "Ада", "2027-06-15", "2027-06-28")
			if got.Subject == "" || got.Body == "" || got.HTML == "" {
				t.Fatalf("%s %s empty", kind, locale)
			}
			if !contains(got.HTML, "#f4f1eb") || !contains(got.HTML, "#d71015") || !contains(got.HTML, "#fdc000") {
				t.Fatalf("%s %s palette", kind, locale)
			}
			if contains(got.HTML, "background:#0C1013") || contains(got.HTML, "background:#000") {
				t.Fatalf("%s %s dark page", kind, locale)
			}
		}
	}
	ru := Compose(KindSubmitted, "ru", "Ада", "2027-06-15", "2027-06-28")
	en := Compose(KindSubmitted, "en", "Ada", "2027-06-15", "2027-06-28")
	if ru.Subject == en.Subject || !contains(ru.HTML, "Сотруднику") || !contains(en.HTML, "Employee") {
		t.Fatalf("ru %q en %q", ru.Subject, en.Subject)
	}
	mgr := Render(Vars{Kind: KindApprovalManager, Locale: "ru", Employee: "Ада", Start: "2027-06-15", End: "2027-06-28", Type: "day_off", Role: "manager"})
	if !contains(mgr.Body, "отгул") || !contains(mgr.HTML, "Руководителю") {
		t.Fatal(mgr.Body)
	}
	raw := formatMessage(Message{From: "hr@example.com", To: "a@example.com", Subject: "Заявка", Body: "Текст", HTML: ru.HTML})
	if !contains(raw, "multipart/alternative") || !contains(raw, "text/html") || !contains(raw, "Текст") || !contains(raw, "#d71015") {
		t.Fatal(raw)
	}
}

func contains(s, part string) bool {
	return len(s) >= len(part) && (s == part || len(part) == 0 || indexOf(s, part) >= 0)
}

func indexOf(s, part string) int {
	for i := 0; i+len(part) <= len(s); i++ {
		if s[i:i+len(part)] == part {
			return i
		}
	}
	return -1
}
