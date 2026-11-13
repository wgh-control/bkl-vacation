package notify

// Sample is one rendered template, used to preview the mail.
type Sample struct {
	Name string
	HTML string
}

// Samples renders every employee, manager, and HR template in Russian and English.
func Samples() []Sample {
	type row struct {
		name string
		vars Vars
	}
	rows := []row{
		{"employee-submitted", Vars{Kind: KindSubmitted, Type: "vacation", Role: "employee"}},
		{"manager-approval-needed", Vars{Kind: KindApprovalManager, Type: "vacation", Role: "manager"}},
		{"hr-approval-needed", Vars{Kind: KindApprovalHR, Type: "sick", Role: "hr"}},
		{"employee-approved", Vars{Kind: KindApproved, Type: "day_off", Role: "employee"}},
		{"employee-declined", Vars{Kind: KindDeclined, Type: "vacation", Role: "employee"}},
		{"employee-notice-14", Vars{Kind: KindNotice14, Type: "vacation", Role: "employee"}},
		{"employee-confirm", Vars{Kind: KindConfirm5, Type: "vacation", Role: "employee"}},
		{"employee-remind-3", Vars{Kind: KindRemind3Employee, Type: "vacation", Role: "employee"}},
		{"manager-remind-3", Vars{Kind: KindRemind3Manager, Type: "vacation", Role: "manager"}},
		{"hr-remind-3", Vars{Kind: KindRemind3HR, Type: "vacation", Role: "hr"}},
		{"hr-to-process", Vars{Kind: KindConfirmedHR, Type: "vacation", Role: "hr"}},
		{"employee-processed", Vars{Kind: KindProcessedEmployee, Type: "vacation", Role: "employee"}},
	}
	var out []Sample
	for _, r := range rows {
		for _, loc := range []string{"ru", "en"} {
			v := r.vars
			v.Locale = loc
			v.Start = "2027-06-15"
			v.End = "2027-06-28"
			if loc == "ru" {
				v.Employee = "Ада Соколова"
			} else {
				v.Employee = "Ada Sokolova"
			}
			if r.name == "employee-submitted" {
				v.Moved = true
			}
			out = append(out, Sample{Name: r.name + "-" + loc, HTML: Render(v).HTML})
		}
	}
	return out
}
