package portal

// Views are the JSON the HTTP API returns. Field names are the contract in the OpenAPI spec.

type Ref struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Profile struct {
	ID              string `json:"id"`
	Subject         string `json:"subject"`
	DisplayName     string `json:"display_name"`
	Email           string `json:"email"`
	Locale          string `json:"locale"`
	EmployeeNumber  string `json:"employee_number"`
	Position        string `json:"position"`
	Department      *Ref   `json:"department,omitempty"`
	Manager         *Ref   `json:"manager,omitempty"`
	HireDate        string `json:"hire_date,omitempty"`
	Active          bool   `json:"active"`
	HR              bool   `json:"hr"`
	ManagerOfOthers bool   `json:"manager_of_others"`
	ExtraDays       int    `json:"extra_days"`
	EarlyLeave      bool   `json:"early_leave"`
}

type WorkingYearView struct {
	StartsOn           string  `json:"starts_on"`
	EndsOn             string  `json:"ends_on"`
	GrantedDays        int     `json:"granted_days"`
	ExtraDays          int     `json:"extra_days"`
	AccruedDays        float64 `json:"accrued_days"`
	AccruedNumerator   int     `json:"accrued_numerator"`
	AccruedDenominator int     `json:"accrued_denominator"`
	AccruedAdjustment  float64 `json:"accrued_adjustment"`
	UsedDays           int     `json:"used_days"`
	ReservedDays       int     `json:"reserved_days"`
	FreeDays           int     `json:"free_days"`
	AdvanceDays        float64 `json:"advance_days"`
	StreakSatisfied    bool    `json:"streak_satisfied"`
	StreakPossible     bool    `json:"streak_possible"`
	CarryoverWarning   bool    `json:"carryover_warning"`
	CarryoverDeadline  string  `json:"carryover_deadline"`
	TwoYearWarning     bool    `json:"two_year_warning"`
	UnpaidCalendarDays int     `json:"unpaid_calendar_days"`
	UnpaidExcessDays   int     `json:"unpaid_excess_days"`
}

type YearEntry struct {
	RequestID  string `json:"request_id"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	StartsOn   string `json:"starts_on"`
	EndsOn     string `json:"ends_on"`
	Days       int    `json:"days"`
	Bucket     string `json:"bucket"`
	Reschedule bool   `json:"reschedule"`
}

// YearDays is one calendar year of the employee client area.
// planned_days, approved_days, and moved_days are vacation days.
// moved_days counts a reschedule's new dates and also overlaps planned or approved.
// moved_away_days counts dates that left this year because a reschedule was approved.
// Balances themselves stay on the working year.
type YearDays struct {
	Year          int         `json:"year"`
	Relation      string      `json:"relation"`
	PlannedDays   int         `json:"planned_days"`
	ApprovedDays  int         `json:"approved_days"`
	MovedDays     int         `json:"moved_days"`
	MovedAwayDays int         `json:"moved_away_days"`
	DayOffDays    int         `json:"day_off_days"`
	SickDays      int         `json:"sick_days"`
	Entries       []YearEntry `json:"entries"`
}

type Balance struct {
	WorkingYears  []WorkingYearView `json:"working_years"`
	CalendarYears []YearDays        `json:"calendar_years"`
}

type YearSummary struct {
	AsOf  string     `json:"as_of"`
	Years []YearDays `json:"years"`
}

type RequestView struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	Status             string `json:"status"`
	EmployeeID         string `json:"employee_id"`
	StartsOn           string `json:"starts_on"`
	EndsOn             string `json:"ends_on"`
	CountedDays        int    `json:"counted_days"`
	ReasonCode         string `json:"reason_code,omitempty"`
	CertificateNumber  string `json:"certificate_number,omitempty"`
	ReplacesRequestID  string `json:"replaces_request_id,omitempty"`
	SuccessorRequestID string `json:"successor_request_id,omitempty"`
	CancelReason       string `json:"cancel_reason,omitempty"`
	OverlapChoice      string `json:"overlap_choice,omitempty"`
	ManagerVacant      bool   `json:"manager_vacant"`
	SelfApproval       bool   `json:"self_approval"`
	NeedsConfirmation  bool   `json:"needs_confirmation"`
	PayByWarning       bool   `json:"pay_by_warning"`
	PayByDate          string `json:"pay_by_date,omitempty"`
	StatutoryPayByDate string `json:"statutory_pay_by_date,omitempty"`
	WorkingYearStart   string `json:"working_year_start,omitempty"`
	WorkingYearEnd     string `json:"working_year_end,omitempty"`
	AcknowledgedAt     string `json:"acknowledged_at,omitempty"`
	ConfirmedAt        string `json:"confirmed_at,omitempty"`
	ProcessedAt        string `json:"processed_at,omitempty"`
	ProcessedBy        string `json:"processed_by,omitempty"`
	CreatedAt          string `json:"created_at"`
}

type DecisionView struct {
	Decision string `json:"decision"`
	Comment  string `json:"comment,omitempty"`
	At       string `json:"at,omitempty"`
	ActorID  string `json:"actor_id,omitempty"`
}

type QueueItem struct {
	RequestView
	EmployeeName    string        `json:"employee_name"`
	EmployeeNumber  string        `json:"employee_number,omitempty"`
	Department      string        `json:"department,omitempty"`
	Position        string        `json:"position,omitempty"`
	ManagerDecision *DecisionView `json:"manager_decision,omitempty"`
}

type NoticeView struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	RequestID string `json:"request_id,omitempty"`
	CreatedAt string `json:"created_at"`
	ReadAt    string `json:"read_at,omitempty"`
}

type ReasonView struct {
	Code             string `json:"code"`
	Unpaid           bool   `json:"unpaid"`
	ManagerMustGrant bool   `json:"manager_must_grant"`
	MaxCalendarDays  int    `json:"max_calendar_days,omitempty"`
}

type CalendarDayView struct {
	Date   string `json:"date"`
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Source string `json:"source"`
}

type WarningView struct {
	EmployeeID   string `json:"employee_id"`
	EmployeeName string `json:"employee_name"`
	Department   string `json:"department,omitempty"`
	Code         string `json:"code"`
	WorkingYear  string `json:"working_year_start"`
	Message      string `json:"message"`
}

type ScheduleView struct {
	Year     int    `json:"year"`
	Locked   bool   `json:"locked"`
	LockedBy string `json:"locked_by,omitempty"`
	LockedAt string `json:"locked_at,omitempty"`
}

type File struct {
	Name        string
	ContentType string
	Body        []byte
}

type CreateInput struct {
	Type              string
	StartsOn          string
	EndsOn            string
	CountedDays       int
	ReasonCode        string
	CertificateNumber string
	OverlapChoice     string
}

type DecisionInput struct {
	Decision string
	Comment  string
}
