// Package model holds the portal's stored records. JSON for the HTTP API
// is shaped in the portal views, not on these types.
package model

import "time"

type RequestType string

const (
	TypeVacation RequestType = "vacation"
	TypeDayOff   RequestType = "day_off"
	TypeSick     RequestType = "sick"
)

type Status string

const (
	StatusDraft                Status = "draft"
	StatusPendingManager       Status = "pending_manager"
	StatusPendingHR            Status = "pending_hr"
	StatusScheduled            Status = "scheduled"
	StatusAwaitingConfirmation Status = "awaiting_confirmation"
	StatusConfirmed            Status = "confirmed"
	StatusProcessed            Status = "processed"
	StatusRejected             Status = "rejected"
	StatusCancelled            Status = "cancelled"
)

const CancelRescheduled = "rescheduled"

type Employee struct {
	ID             string
	Subject        string
	DisplayName    string
	Email          string
	Locale         string
	LocaleChosen   bool
	EmployeeNumber string
	Position       string
	DepartmentID   string
	ManagerID      string
	ManagerRaw     string
	HireDate       *time.Time
	Active         bool
	HR             bool
	ExtraDays      int
	EarlyLeave     bool
	Raw            []byte
	LastSyncAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type OrgUnit struct {
	ID         string
	ExternalID string
	Name       string
	ParentID   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type LeaveYear struct {
	ID                 string
	EmployeeID         string
	StartsOn           time.Time
	EndsOn             time.Time
	GrantedDays        int
	ExtraDays          int
	AccruedAdjustment  float64
	UnpaidCalendarDays int
	ExcessUnpaidDays   int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type Request struct {
	ID                string
	EmployeeID        string
	LeaveYearID       string
	Type              RequestType
	Status            Status
	StartsOn          time.Time
	EndsOn            time.Time
	CountedDays       int
	ReasonCode        string
	CertificateNumber string
	ReplacesID        string
	SuccessorID       string
	CancelReason      string
	OverlapChoice     string
	ManagerVacant     bool
	SelfApproval      bool
	NeedsConfirmation bool
	PayByWarning      bool
	AcknowledgedAt    *time.Time
	ConfirmedAt       *time.Time
	ProcessedAt       *time.Time
	ProcessedBy       string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Step struct {
	ID                 string
	RequestID          string
	Sequence           int
	Role               string
	ExpectedEmployeeID string
	Decision           string
	Comment            string
	DecidedAt          *time.Time
	ActorID            string
}

type CalendarDay struct {
	Date      time.Time
	Kind      string
	Name      string
	Source    string
	UpdatedAt time.Time
}

type Notification struct {
	ID                  string
	RequestID           string
	RecipientEmployeeID string
	Kind                string
	RecipientEmail      string
	Language            string
	DueAt               time.Time
	SentAt              *time.Time
	LastError           string
	NoticeCreated       bool
	CreatedAt           time.Time
}

type Notice struct {
	ID             string
	EmployeeID     string
	RequestID      string
	NotificationID string
	Kind           string
	Title          string
	Body           string
	CreatedAt      time.Time
	ReadAt         *time.Time
}

type Audit struct {
	ID           string
	ActorID      string
	RequestID    string
	EmployeeID   string
	Action       string
	BeforeStatus string
	AfterStatus  string
	Detail       []byte
	CreatedAt    time.Time
}

type ScheduleLock struct {
	Year     int
	LockedBy string
	LockedAt time.Time
}

type RequestFilter struct {
	EmployeeID    string
	ManagerID     string
	DepartmentID  string
	Statuses      []string
	EndsOnOrAfter *time.Time
	OverlapFrom   *time.Time
	OverlapTo     *time.Time
	Limit         int
}
