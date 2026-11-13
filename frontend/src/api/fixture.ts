import {
  addDays,
  calendarMap,
  countRange,
  countedOverlap,
  daysUntil,
  eachDate,
  federalHolidayCode,
  isWeekend,
  rangesOverlap,
  workingYearContaining,
} from '../domain/dates'
import { UNPAID_REASONS, confirmationState, dayOffHasLeadTime, previewLeave } from '../domain/leave'
import { ApiError } from './errors'
import { buildPdf, buildXls } from './reportFile'
import type {
  ApprovalItem,
  Balance,
  CalendarDay,
  CreateRequestBody,
  Department,
  ExportQuery,
  HrQueue,
  Language,
  LeaveRequest,
  Me,
  OverlapChoice,
  PersonRef,
  RequestStatus,
  Role,
  VacationApi,
} from './types'

/**
 * In-memory API for the local UI preview and tests.
 * It follows the same VacationApi interface as the HTTP client.
 * The development server is the only place the portal mounts it.
 */

type StoredPerson = {
  id: string
  subject: string
  displayName: string
  email: string
  language: Language
  employeeNumber: string
  position: string
  departmentId: string
  department: string
  hireDate: string
  managerId: string | null
  roles: Role[]
  active: boolean
}

const REPORT_COLUMNS: Record<Language, string[]> = {
  ru: [
    'Тип',
    'Статус',
    'Сотрудник',
    'Табельный номер',
    'Подразделение',
    'Должность',
    'Руководитель',
    'Начало',
    'Окончание',
    'Дней',
    'Номер больничного',
    'Причина отгула',
    'Подтверждено',
    'Оформлено',
  ],
  en: [
    'Type',
    'Status',
    'Employee',
    'Employee number',
    'Department',
    'Position',
    'Manager',
    'Start',
    'End',
    'Counted days',
    'Sick-certificate number',
    'Day-off reason',
    'Confirmed at',
    'Processed at',
  ],
}

const TYPE_LABEL: Record<Language, Record<LeaveRequest['type'], string>> = {
  ru: { vacation: 'Отпуск', day_off: 'Отгул', sick: 'Больничный' },
  en: { vacation: 'Vacation', day_off: 'Day off', sick: 'Sick day' },
}

const STATUS_LABEL: Record<Language, Record<RequestStatus, string>> = {
  ru: {
    draft: 'Черновик',
    pending_manager: 'У руководителя',
    pending_hr: 'У кадров',
    scheduled: 'В графике',
    awaiting_confirmation: 'Ждёт подтверждения',
    confirmed: 'Подтверждён',
    processed: 'Оформлен',
    rejected: 'Отклонён',
    cancelled: 'Отменён',
  },
  en: {
    draft: 'Draft',
    pending_manager: 'With manager',
    pending_hr: 'With HR',
    scheduled: 'Scheduled',
    awaiting_confirmation: 'Awaiting confirmation',
    confirmed: 'Confirmed',
    processed: 'Processed',
    rejected: 'Rejected',
    cancelled: 'Cancelled',
  },
}

function clone<T>(value: T): T {
  return structuredClone(value)
}

function personRef(person: StoredPerson): PersonRef {
  return {
    id: person.id,
    displayName: person.displayName,
    employeeNumber: person.employeeNumber,
    position: person.position,
    department: person.department,
    departmentId: person.departmentId,
  }
}

export type FixtureApi = VacationApi & {
  today: string
  setRoles(roles: Role[]): void
  setCurrentUser(personId: string): void
}

export function createFixtureApi(options?: { today?: string }): FixtureApi {
  const today = options?.today ?? new Date().toISOString().slice(0, 10)
  const people = new Map<string, StoredPerson>()
  const requests: LeaveRequest[] = []
  let sequence = 1
  let currentId = 'emp-anna'

  function addPerson(person: StoredPerson): void {
    people.set(person.id, person)
  }

  addPerson({
    id: 'emp-anna',
    subject: 'fixture-anna',
    displayName: 'Соколова Анна',
    email: 'anna.sokolova@localhost',
    language: 'ru',
    employeeNumber: '1001',
    position: 'Специалист по кадрам',
    departmentId: 'dept-people',
    department: 'Персонал',
    hireDate: '2019-04-15',
    managerId: 'emp-oleg',
    roles: ['employee', 'manager', 'hr'],
    active: true,
  })
  addPerson({
    id: 'emp-oleg',
    subject: 'fixture-oleg',
    displayName: 'Кузнецов Олег',
    email: 'oleg.kuznetsov@localhost',
    language: 'ru',
    employeeNumber: '1008',
    position: 'Руководитель отдела',
    departmentId: 'dept-people',
    department: 'Персонал',
    hireDate: '2014-02-01',
    managerId: null,
    roles: ['employee', 'manager'],
    active: true,
  })
  addPerson({
    id: 'emp-ivan',
    subject: 'fixture-ivan',
    displayName: 'Иванов Пётр',
    email: 'petr.ivanov@localhost',
    language: 'ru',
    employeeNumber: '1044',
    position: 'Инженер',
    departmentId: 'dept-people',
    department: 'Персонал',
    hireDate: '2021-09-01',
    managerId: 'emp-anna',
    roles: ['employee'],
    active: true,
  })
  addPerson({
    id: 'emp-maria',
    subject: 'fixture-maria',
    displayName: 'Морозова Мария',
    email: 'maria.morozova@localhost',
    language: 'ru',
    employeeNumber: '1102',
    position: 'Бухгалтер',
    departmentId: 'dept-finance',
    department: 'Финансы',
    hireDate: '2018-11-20',
    managerId: 'emp-finance',
    roles: ['employee'],
    active: true,
  })
  addPerson({
    id: 'emp-finance',
    subject: 'fixture-finance',
    displayName: 'Орлова Елена',
    email: 'elena.orlova@localhost',
    language: 'ru',
    employeeNumber: '1090',
    position: 'Главный бухгалтер',
    departmentId: 'dept-finance',
    department: 'Финансы',
    hireDate: '2012-05-10',
    managerId: null,
    roles: ['employee', 'manager'],
    active: true,
  })

  function mePerson(): StoredPerson {
    const person = people.get(currentId)
    if (!person) throw new ApiError('unauthorized', 401)
    return person
  }

  function requireRole(role: Role): StoredPerson {
    const person = mePerson()
    if (!person.roles.includes(role)) throw new ApiError('forbidden', 403)
    return person
  }

  function stamp(request: LeaveRequest): void {
    if (request.status === 'pending_manager') {
      request.nextActor = 'manager'
      request.pendingStepId = `step-${request.id}-manager`
    } else if (request.status === 'pending_hr') {
      request.nextActor = 'hr'
      request.pendingStepId = `step-${request.id}-hr`
    } else if (request.status === 'awaiting_confirmation') {
      request.nextActor = 'employee'
      request.pendingStepId = undefined
    } else {
      request.nextActor = null
      request.pendingStepId = undefined
    }
    if (request.type === 'vacation' && request.start) {
      const pay = addDays(request.start, -4)
      request.payByDate = pay
      request.payByWarning = pay < today
    }
  }

  function seed(
    partial: Omit<LeaveRequest, 'countedDays' | 'payByDate' | 'payByWarning' | 'nextActor' | 'pendingStepId'> & {
      countedDays?: number
    },
  ): LeaveRequest {
    const request: LeaveRequest = {
      ...partial,
      countedDays: partial.countedDays ?? countRange(partial.start, partial.end).countedDays,
    }
    stamp(request)
    requests.push(request)
    return request
  }

  const year = Number(today.slice(0, 4))
  seed({
    id: 'req-past',
    type: 'vacation',
    status: 'processed',
    person: personRef(people.get('emp-anna')!),
    start: `${year - 1}-06-01`,
    end: `${year - 1}-06-15`,
    createdAt: `${year - 1}-03-01T09:00:00`,
    confirmedAt: `${year - 1}-05-20T09:00:00`,
    processedAt: `${year - 1}-05-22T09:00:00`,
    acknowledgedAt: `${year - 1}-05-10T09:00:00`,
  })
  const original = seed({
    id: 'req-move-old',
    type: 'vacation',
    status: 'scheduled',
    person: personRef(people.get('emp-anna')!),
    start: addDays(today, -50),
    end: addDays(today, -46),
    replacedByRequestId: 'req-move-new',
    createdAt: addDays(today, -80) + 'T09:00:00',
    acknowledgedAt: addDays(today, -70) + 'T09:00:00',
  })
  seed({
    id: 'req-move-new',
    type: 'vacation',
    status: 'pending_hr',
    person: personRef(people.get('emp-anna')!),
    start: addDays(today, 60),
    end: addDays(today, 64),
    replacesRequestId: original.id,
    createdAt: addDays(today, -2) + 'T09:00:00',
    managerDecision: {
      person: personRef(people.get('emp-oleg')!),
      decision: 'approve',
      at: addDays(today, -1) + 'T10:00:00',
    },
  })
  seed({
    id: 'req-confirm',
    type: 'vacation',
    status: 'awaiting_confirmation',
    person: personRef(people.get('emp-anna')!),
    start: addDays(today, 4),
    end: addDays(today, 7),
    createdAt: addDays(today, -20) + 'T09:00:00',
    managerDecision: {
      person: personRef(people.get('emp-oleg')!),
      decision: 'approve',
      at: addDays(today, -10) + 'T10:00:00',
    },
    hrDecision: {
      person: personRef(people.get('emp-anna')!),
      decision: 'approve',
      at: addDays(today, -8) + 'T10:00:00',
    },
  })
  seed({
    id: 'req-processed-ready',
    type: 'vacation',
    status: 'confirmed',
    person: personRef(people.get('emp-anna')!),
    start: addDays(today, 24),
    end: addDays(today, 28),
    createdAt: addDays(today, -15) + 'T09:00:00',
    confirmedAt: addDays(today, -1) + 'T12:00:00',
    acknowledgedAt: addDays(today, -1) + 'T12:00:00',
  })
  seed({
    id: 'req-anna-sick',
    type: 'sick',
    status: 'pending_manager',
    person: personRef(people.get('emp-anna')!),
    start: addDays(today, 25),
    end: addDays(today, 26),
    createdAt: today + 'T09:00:00',
    overlap: { vacationRequestId: 'req-processed-ready', days: 2 },
  })
  seed({
    id: 'req-dayoff',
    type: 'day_off',
    status: 'pending_manager',
    person: personRef(people.get('emp-anna')!),
    start: addDays(today, 45),
    end: addDays(today, 45),
    reasonCode: 'unpaid_agreement',
    createdAt: today + 'T09:00:00',
  })
  seed({
    id: 'req-ivan-vacation',
    type: 'vacation',
    status: 'pending_manager',
    person: personRef(people.get('emp-ivan')!),
    start: addDays(today, 18),
    end: addDays(today, 29),
    createdAt: addDays(today, -1) + 'T09:00:00',
  })
  seed({
    id: 'req-ivan-sick',
    type: 'sick',
    status: 'pending_hr',
    person: personRef(people.get('emp-ivan')!),
    start: addDays(today, 8),
    end: addDays(today, 9),
    certificateNumber: '123456789012',
    createdAt: today + 'T08:00:00',
    managerDecision: {
      person: personRef(people.get('emp-anna')!),
      decision: 'approve',
      at: today + 'T09:30:00',
    },
  })
  seed({
    id: 'req-ivan-dayoff',
    type: 'day_off',
    status: 'pending_hr',
    person: personRef(people.get('emp-ivan')!),
    start: `${year + 1}-03-10`,
    end: `${year + 1}-03-10`,
    reasonCode: 'compensatory_overtime',
    createdAt: today + 'T09:00:00',
    managerDecision: {
      person: personRef(people.get('emp-anna')!),
      decision: 'approve',
      at: today + 'T11:00:00',
    },
  })
  seed({
    id: 'req-maria-vacation',
    type: 'vacation',
    status: 'pending_hr',
    person: personRef(people.get('emp-maria')!),
    start: `${year + 1}-07-06`,
    end: `${year + 1}-07-19`,
    createdAt: addDays(today, -3) + 'T09:00:00',
    managerDecision: {
      person: personRef(people.get('emp-finance')!),
      decision: 'approve',
      comment: 'Согласовано',
      at: addDays(today, -1) + 'T15:00:00',
    },
  })
  seed({
    id: 'req-maria-sick',
    type: 'sick',
    status: 'pending_hr',
    person: personRef(people.get('emp-maria')!),
    start: addDays(today, 12),
    end: addDays(today, 13),
    createdAt: today + 'T09:00:00',
    managerDecision: {
      person: personRef(people.get('emp-finance')!),
      decision: 'approve',
      at: today + 'T10:00:00',
    },
  })

  function currentMe(): Me {
    const person = mePerson()
    const manager = person.managerId ? people.get(person.managerId) : undefined
    return {
      ...personRef(person),
      subject: person.subject,
      email: person.email,
      language: person.language,
      hireDate: person.hireDate,
      manager: manager ? personRef(manager) : null,
      roles: [...person.roles],
      active: person.active,
    }
  }

  function owns(request: LeaveRequest): boolean {
    return request.person.id === mePerson().id
  }

  function canRead(request: LeaveRequest): boolean {
    const person = mePerson()
    if (owns(request)) return true
    if (person.roles.includes('hr')) return true
    const owner = people.get(request.person.id)
    return Boolean(person.roles.includes('manager') && owner?.managerId === person.id)
  }

  function findRequest(id: string): LeaveRequest {
    const request = requests.find((item) => item.id === id)
    if (!request) throw new ApiError('not_found', 404)
    if (!canRead(request)) throw new ApiError('forbidden', 403)
    return request
  }

  function isClosed(status: RequestStatus): boolean {
    return status === 'draft' || status === 'rejected' || status === 'cancelled'
  }

  function countsTowardBalance(request: LeaveRequest): boolean {
    if (request.type !== 'vacation' || isClosed(request.status)) return false
    const successor = request.replacedByRequestId
      ? requests.find((item) => item.id === request.replacedByRequestId)
      : undefined
    if (successor && !isClosed(successor.status)) {
      const approved = !['pending_manager', 'pending_hr'].includes(successor.status)
      return !approved
    }
    if (request.replacesRequestId) return !['pending_manager', 'pending_hr'].includes(request.status)
    return true
  }

  function balanceFor(personId: string, on: string, releaseId?: string): Balance {
    const person = people.get(personId)
    if (!person) throw new ApiError('not_found', 404)
    const yearSpan = workingYearContaining(person.hireDate, on)
    let used = 0
    let reserved = 0
    let hasStreak = false
    for (const request of requests) {
      if (request.person.id !== personId || request.id === releaseId) continue
      if (request.start < yearSpan.start || request.start > yearSpan.end) continue
      if (request.type === 'vacation' && !isClosed(request.status) && request.countedDays >= 14) {
        const approved = ['scheduled', 'awaiting_confirmation', 'confirmed', 'processed'].includes(request.status)
        if (approved && countsTowardBalance(request)) hasStreak = true
      }
      if (!countsTowardBalance(request)) continue
      if (request.end < on) used += request.countedDays
      else reserved += request.countedDays
    }
    const granted = 28
    const extra = 0
    const free = Math.max(0, granted + extra - used - reserved)
    let unpaid = 0
    for (const request of requests) {
      if (request.person.id !== personId || request.type !== 'day_off') continue
      if (!request.reasonCode || !UNPAID_REASONS.has(request.reasonCode) || isClosed(request.status)) continue
      const start = request.start > yearSpan.start ? request.start : yearSpan.start
      const end = request.end < yearSpan.end ? request.end : yearSpan.end
      if (end >= start) unpaid += eachDate(start, end).length
    }
    const hireMonths = monthsBetweenSafe(person.hireDate, on)
    const warnings: string[] = []
    if (unpaid > 14) warnings.push('unpaid_day_off_excess')
    return {
      workingYearStart: yearSpan.start,
      workingYearEnd: yearSpan.end,
      grantedDays: granted,
      extraDays: extra,
      usedDays: used,
      reservedDays: reserved,
      freeDays: free,
      carryoverDays: 0,
      accruedAdjustment: 0,
      advanceDays: Math.max(0, used + reserved - granted - extra),
      hasStreak,
      streakPossible: hasStreak || free >= 14,
      unpaidDayOffDays: unpaid,
      withinFirstSixMonths: hireMonths < 6,
      warnings,
    }
  }

  function calendar(from: string, to: string): CalendarDay[] {
    return eachDate(from, to).map((date) => {
      const code = federalHolidayCode(date)
      if (code) return { date, kind: 'holiday', name: code }
      if (isWeekend(date)) return { date, kind: 'weekend' }
      return { date, kind: 'work' }
    })
  }

  function existingOf(personId: string) {
    return requests
      .filter((request) => request.person.id === personId)
      .map((request) => ({
        id: request.id,
        type: request.type,
        status: request.status,
        start: request.start,
        end: request.end,
      }))
  }

  function place(person: StoredPerson, body: CreateRequestBody, links?: { replacesRequestId?: string; ignoreIds?: string[] }): LeaveRequest {
    if (body.type === 'day_off' && !body.reasonCode) throw new ApiError('validation', 400)
    const balance = balanceFor(person.id, body.start || today, links?.replacesRequestId)
    const preview = previewLeave({
      type: body.type,
      start: body.start,
      end: body.end,
      reasonCode: body.reasonCode,
      calendar: calendarMap(calendar(body.start, body.end)),
      existing: existingOf(person.id),
      ignoreIds: links?.ignoreIds,
      balance,
      today,
    })
    const reject = preview.issues.find((issue) => issue.level === 'reject')
    if (reject) throw new ApiError(reject.code, 400)
    const manager = person.managerId ? people.get(person.managerId) : undefined
    const vacant = !manager || manager.id === person.id
    const request: LeaveRequest = {
      id: `req-${sequence++}`,
      type: body.type,
      status: vacant ? 'pending_hr' : 'pending_manager',
      person: personRef(person),
      start: body.start,
      end: body.end,
      countedDays: preview.countedDays,
      reasonCode: body.reasonCode,
      certificateNumber: body.certificateNumber,
      comment: body.comment,
      replacesRequestId: links?.replacesRequestId,
      createdAt: `${today}T09:00:00`,
      routedToHrBecauseManagerVacant: vacant,
    }
    if (body.type === 'sick') {
      let best: { id: string; days: number } | undefined
      for (const other of requests) {
        if (other.person.id !== person.id || other.type !== 'vacation' || isClosed(other.status)) continue
        const days = countedOverlap(body.start, body.end, other.start, other.end, calendarMap(calendar(body.start, body.end)))
        if (days > 0 && (!best || days > best.days)) best = { id: other.id, days }
      }
      if (best) request.overlap = { vacationRequestId: best.id, days: best.days }
    }
    stamp(request)
    requests.push(request)
    return request
  }

  function approveHr(request: LeaveRequest, actor: StoredPerson, comment?: string): void {
    if (request.type === 'sick' && !request.certificateNumber) throw new ApiError('certificate_required', 400)
    request.hrDecision = { person: personRef(actor), decision: 'approve', comment, at: `${today}T12:00:00` }
    if (request.replacesRequestId) {
      const original = requests.find((item) => item.id === request.replacesRequestId)
      if (original) {
        original.status = 'cancelled'
        original.processedAt = undefined
        original.processedBy = undefined
        stamp(original)
      }
    }
    request.processedAt = undefined
    request.processedBy = undefined
    const days = daysUntil(today, request.start)
    const inWindow = days >= 0 && days <= 5
    const confirm =
      (request.type === 'vacation' && inWindow) ||
      (request.type === 'day_off' && dayOffHasLeadTime(request) && inWindow)
    request.status = confirm ? 'awaiting_confirmation' : 'scheduled'
    stamp(request)
  }

  const api: FixtureApi = {
    today,
    setRoles(roles: Role[]) {
      const person = mePerson()
      const next = new Set(roles)
      next.add('employee')
      person.roles = (['employee', 'manager', 'hr'] as Role[]).filter((role) => next.has(role))
    },
    setCurrentUser(personId: string) {
      if (!people.has(personId)) throw new ApiError('not_found', 404)
      currentId = personId
    },
    async getMe() {
      return currentMe()
    },
    async updateLanguage(language: Language) {
      mePerson().language = language
      return currentMe()
    },
    async getBalance(on: string) {
      return balanceFor(mePerson().id, on)
    },
    async getCalendar(from: string, to: string) {
      return calendar(from, to)
    },
    async getRequests(query = {}) {
      const scope = query.scope ?? 'mine'
      const person = mePerson()
      if (scope === 'reports') requireRole('manager')
      if (scope === 'all') requireRole('hr')
      return requests
        .filter((request) => {
          if (scope === 'mine' && request.person.id !== person.id) return false
          if (scope === 'reports') {
            const owner = people.get(request.person.id)
            if (owner?.managerId !== person.id) return false
          }
          if (query.from && query.to && !rangesOverlap(request.start, request.end, query.from, query.to)) return false
          return true
        })
        .map((request) => clone(request))
        .sort((a, b) => a.start.localeCompare(b.start))
    },
    async getRequest(id: string) {
      return clone(findRequest(id))
    },
    async createRequest(body: CreateRequestBody) {
      return clone(place(mePerson(), body))
    },
    async cancelRequest(id: string) {
      const request = findRequest(id)
      if (!owns(request)) throw new ApiError('forbidden', 403)
      if (request.status === 'processed') throw new ApiError('conflict', 409)
      if (request.status === 'rejected' || request.status === 'cancelled') throw new ApiError('conflict', 409)
      request.status = 'cancelled'
      stamp(request)
      return clone(request)
    },
    async rescheduleRequest(id, body) {
      const request = findRequest(id)
      if (!owns(request)) throw new ApiError('forbidden', 403)
      if (request.status === 'rejected' || request.status === 'cancelled' || request.status === 'draft') {
        throw new ApiError('conflict', 409)
      }
      const successor = place(
        mePerson(),
        {
          type: request.type,
          start: body.start,
          end: body.end,
          reasonCode: request.reasonCode,
          certificateNumber: request.certificateNumber,
          comment: body.comment,
        },
        { replacesRequestId: request.id, ignoreIds: [request.id] },
      )
      request.replacedByRequestId = successor.id
      return clone(successor)
    },
    async confirmRequest(id: string) {
      const request = findRequest(id)
      if (!owns(request)) throw new ApiError('forbidden', 403)
      const state = confirmationState(request, today)
      if (state !== 'ready' && state !== 'passed') throw new ApiError('conflict', 409)
      request.status = 'confirmed'
      request.confirmedAt = `${today}T12:00:00`
      stamp(request)
      return clone(request)
    },
    async acknowledgeRequest(id: string) {
      const request = findRequest(id)
      if (!owns(request)) throw new ApiError('forbidden', 403)
      request.acknowledgedAt = `${today}T12:00:00`
      return clone(request)
    },
    async setCertificate(id, certificateNumber) {
      const request = findRequest(id)
      if (!owns(request) || request.type !== 'sick') throw new ApiError('validation', 400)
      if (!certificateNumber.trim()) throw new ApiError('validation', 400)
      request.certificateNumber = certificateNumber.trim()
      return clone(request)
    },
    async chooseOverlap(id: string, choice: OverlapChoice) {
      const request = findRequest(id)
      if (!owns(request) || !request.overlap) throw new ApiError('conflict', 409)
      request.overlap.choice = choice
      if (choice === 'extend') {
        const vacation = requests.find((item) => item.id === request.overlap?.vacationRequestId)
        if (vacation) {
          let extra = request.overlap.days
          let end = vacation.end
          while (extra > 0) {
            end = addDays(end, 1)
            if (!federalHolidayCode(end)) extra -= 1
          }
          vacation.end = end
          vacation.countedDays = countRange(vacation.start, vacation.end).countedDays
          stamp(vacation)
        }
      }
      return clone(request)
    },
    async getApprovals() {
      const person = mePerson()
      const items: ApprovalItem[] = []
      for (const request of requests) {
        if (request.status === 'pending_manager' && person.roles.includes('manager')) {
          const owner = people.get(request.person.id)
          if (owner?.managerId === person.id && request.pendingStepId) {
            items.push({ id: request.pendingStepId, request: clone(request), role: 'manager' })
          }
        }
        if (request.status === 'pending_hr' && person.roles.includes('hr') && request.pendingStepId) {
          items.push({
            id: request.pendingStepId,
            request: clone(request),
            role: 'hr',
            managerVacant: request.routedToHrBecauseManagerVacant,
          })
        }
      }
      return items.sort((a, b) => a.request.start.localeCompare(b.request.start))
    },
    async decide(stepId, body) {
      const request = requests.find((item) => item.pendingStepId === stepId)
      if (!request) throw new ApiError('not_found', 404)
      const person = mePerson()
      if (request.status === 'pending_manager') {
        requireRole('manager')
        const owner = people.get(request.person.id)
        if (owner?.managerId !== person.id) throw new ApiError('forbidden', 403)
        request.managerDecision = {
          person: personRef(person),
          decision: body.decision,
          comment: body.comment,
          at: `${today}T12:00:00`,
        }
        request.status = body.decision === 'approve' ? 'pending_hr' : 'rejected'
      } else if (request.status === 'pending_hr') {
        requireRole('hr')
        if (body.decision === 'reject') {
          request.hrDecision = {
            person: personRef(person),
            decision: 'reject',
            comment: body.comment,
            at: `${today}T12:00:00`,
          }
          request.status = 'rejected'
        } else {
          approveHr(request, person, body.comment)
          return clone(request)
        }
      } else {
        throw new ApiError('conflict', 409)
      }
      stamp(request)
      return clone(request)
    },
    async getHrQueue() {
      requireRole('hr')
      const queue: HrQueue = {
        pendingApproval: requests
          .filter((request) => request.status === 'pending_hr')
          .map((request) => clone(request))
          .sort((a, b) => a.start.localeCompare(b.start)),
        awaitingProcessed: requests
          .filter((request) => request.type === 'vacation' && request.status === 'confirmed')
          .map((request) => clone(request))
          .sort((a, b) => a.start.localeCompare(b.start)),
      }
      return queue
    },
    async markProcessed(id: string) {
      requireRole('hr')
      const request = requests.find((item) => item.id === id)
      if (!request) throw new ApiError('not_found', 404)
      if (request.type !== 'vacation' || request.status !== 'confirmed') throw new ApiError('conflict', 409)
      request.status = 'processed'
      request.processedAt = `${today}T15:00:00`
      request.processedBy = personRef(mePerson())
      stamp(request)
      return clone(request)
    },
    async exportReport(query: ExportQuery) {
      requireRole('hr')
      if (query.scope === 'person' && !query.personId) throw new ApiError('validation', 400)
      if (query.scope === 'department' && !query.departmentId) throw new ApiError('validation', 400)
      const language = mePerson().language
      const matched = requests.filter((request) => {
        if (isClosed(request.status)) return false
        if (query.scope === 'person' && request.person.id !== query.personId) return false
        if (query.scope === 'department' && request.person.departmentId !== query.departmentId) return false
        if (query.from && query.to && !rangesOverlap(request.start, request.end, query.from, query.to)) return false
        return true
      })
      const headers = REPORT_COLUMNS[language]
      const rows = matched.map((request) => {
        const manager = people.get(people.get(request.person.id)?.managerId ?? '')
        return [
          TYPE_LABEL[language][request.type],
          STATUS_LABEL[language][request.status],
          request.person.displayName,
          request.person.employeeNumber ?? '',
          request.person.department ?? '',
          request.person.position ?? '',
          manager?.displayName ?? '',
          request.start,
          request.end,
          String(request.countedDays),
          request.certificateNumber ?? '',
          request.reasonCode ?? '',
          request.confirmedAt ?? '',
          request.processedAt ?? '',
        ]
      })
      const extension = query.format === 'pdf' ? 'pdf' : 'xls'
      const filename = `bkl-leave-${query.scope}.${extension}`
      if (query.format === 'pdf') {
        const bytes = await buildPdf(language === 'ru' ? 'LEON отпуска' : 'LEON leave', headers, rows)
        return { blob: new Blob([Uint8Array.from(bytes)], { type: 'application/pdf' }), filename }
      }
      const xml = buildXls(headers, rows)
      return {
        blob: new Blob([xml], { type: 'application/vnd.ms-excel' }),
        filename: query.format === 'csv' ? filename.replace(/\.xls$/, '.csv') : filename,
      }
    },
    async listPeople(q?: string) {
      requireRole('hr')
      const needle = q?.trim().toLowerCase()
      return [...people.values()]
        .filter((person) => {
          if (!needle) return true
          return (
            person.displayName.toLowerCase().includes(needle) ||
            person.employeeNumber.includes(needle)
          )
        })
        .map(personRef)
    },
    async listDepartments() {
      requireRole('hr')
      const departments = new Map<string, Department>()
      for (const person of people.values()) {
        departments.set(person.departmentId, { id: person.departmentId, name: person.department })
      }
      return [...departments.values()]
    },
  }

  return api
}

function monthsBetweenSafe(from: string, to: string): number {
  const [fromYear, fromMonth, fromDay] = from.split('-').map(Number)
  const [toYear, toMonth, toDay] = to.split('-').map(Number)
  let months = (toYear - fromYear) * 12 + (toMonth - fromMonth)
  if (toDay < fromDay) months -= 1
  return months
}
