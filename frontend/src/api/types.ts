export type Language = 'ru' | 'en'

export type Role = 'employee' | 'manager' | 'hr'

export type RequestType = 'vacation' | 'day_off' | 'sick'

export type RequestStatus =
  | 'draft'
  | 'pending_manager'
  | 'pending_hr'
  | 'scheduled'
  | 'awaiting_confirmation'
  | 'confirmed'
  | 'processed'
  | 'rejected'
  | 'cancelled'

export type DayOffReason =
  | 'unpaid_agreement'
  | 'unpaid_childbirth'
  | 'unpaid_marriage'
  | 'unpaid_bereavement'
  | 'compensatory_overtime'
  | 'compensatory_weekend'

export type PersonRef = {
  id: string
  displayName: string
  employeeNumber?: string
  position?: string
  department?: string
  departmentId?: string
}

export type Me = PersonRef & {
  subject: string
  email: string
  language: Language
  hireDate?: string
  manager: PersonRef | null
  roles: Role[]
  active: boolean
}

export type Balance = {
  workingYearStart: string
  workingYearEnd: string
  grantedDays: number
  extraDays: number
  usedDays: number
  reservedDays: number
  freeDays: number
  carryoverDays: number
  accruedAdjustment: number
  advanceDays: number
  hasStreak: boolean
  streakPossible: boolean
  unpaidDayOffDays: number
  withinFirstSixMonths: boolean
  warnings: string[]
}

export type CalendarDay = {
  date: string
  kind: 'work' | 'weekend' | 'holiday' | 'transferred'
  name?: string
}

export type Decision = {
  person?: PersonRef
  decision: 'approve' | 'reject'
  comment?: string
  at: string
}

export type OverlapChoice = 'extend' | 'postpone'

export type Overlap = {
  vacationRequestId: string
  days: number
  choice?: OverlapChoice
}

export type LeaveRequest = {
  id: string
  type: RequestType
  status: RequestStatus
  person: PersonRef
  start: string
  end: string
  countedDays: number
  reasonCode?: DayOffReason
  certificateNumber?: string
  replacesRequestId?: string
  replacedByRequestId?: string
  comment?: string
  managerDecision?: Decision
  hrDecision?: Decision
  confirmedAt?: string
  processedAt?: string
  processedBy?: PersonRef
  acknowledgedAt?: string
  payByDate?: string
  payByWarning?: boolean
  createdAt: string
  pendingStepId?: string
  nextActor?: 'manager' | 'hr' | 'employee' | null
  routedToHrBecauseManagerVacant?: boolean
  overlap?: Overlap
}

export type ApprovalItem = {
  id: string
  request: LeaveRequest
  role: 'manager' | 'hr'
  managerChanged?: boolean
  managerVacant?: boolean
}

export type HrQueue = {
  pendingApproval: LeaveRequest[]
  awaitingProcessed: LeaveRequest[]
}

export type Department = {
  id: string
  name: string
}

export type CreateRequestBody = {
  type: RequestType
  start: string
  end: string
  reasonCode?: DayOffReason
  certificateNumber?: string
  comment?: string
}

export type RequestQuery = {
  from?: string
  to?: string
  scope?: 'mine' | 'reports' | 'all'
}

/** pdf and xls are what the portal downloads. csv and xlsx stay available for the API spec. */
export type ExportFormat = 'pdf' | 'xls' | 'csv' | 'xlsx'

export type ExportScope = 'person' | 'department' | 'all'

export type ExportQuery = {
  format: ExportFormat
  scope: ExportScope
  personId?: string
  departmentId?: string
  from?: string
  to?: string
}

export type ReportFile = {
  blob: Blob
  filename: string
}

/**
 * Portal API. Call sites depend on this interface.
 * `createHttpApi` speaks to the Go service. Swap that module when OpenAPI lands.
 */
export type VacationApi = {
  getMe(): Promise<Me>
  updateLanguage(language: Language): Promise<Me>
  getBalance(on: string): Promise<Balance>
  getCalendar(from: string, to: string): Promise<CalendarDay[]>
  getRequests(query?: RequestQuery): Promise<LeaveRequest[]>
  getRequest(id: string): Promise<LeaveRequest>
  createRequest(body: CreateRequestBody): Promise<LeaveRequest>
  cancelRequest(id: string): Promise<LeaveRequest>
  rescheduleRequest(id: string, body: { start: string; end: string; comment?: string }): Promise<LeaveRequest>
  confirmRequest(id: string): Promise<LeaveRequest>
  acknowledgeRequest(id: string): Promise<LeaveRequest>
  setCertificate(id: string, certificateNumber: string): Promise<LeaveRequest>
  chooseOverlap(id: string, choice: OverlapChoice): Promise<LeaveRequest>
  getApprovals(): Promise<ApprovalItem[]>
  decide(stepId: string, body: { decision: 'approve' | 'reject'; comment?: string }): Promise<LeaveRequest>
  getHrQueue(): Promise<HrQueue>
  markProcessed(id: string): Promise<LeaveRequest>
  exportReport(query: ExportQuery): Promise<ReportFile>
  listPeople(query?: string): Promise<PersonRef[]>
  listDepartments(): Promise<Department[]>
}
