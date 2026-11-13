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
