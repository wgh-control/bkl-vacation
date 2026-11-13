import type { DayOffReason, LeaveRequest, RequestStatus, RequestType } from '../api/types'
import {
  type CountedRange,
  type DayKind,
  countRange,
  countedOverlap,
  daysUntil,
  payByDates,
  rangesOverlap,
} from './dates'

export const UNPAID_REASONS = new Set<DayOffReason>([
  'unpaid_agreement',
  'unpaid_childbirth',
  'unpaid_marriage',
  'unpaid_bereavement',
])

export const OPEN_STATUSES = new Set<RequestStatus>([
  'pending_manager',
  'pending_hr',
  'scheduled',
  'awaiting_confirmation',
  'confirmed',
  'processed',
])

const APPROVED = new Set<RequestStatus>([
  'scheduled',
  'awaiting_confirmation',
  'confirmed',
  'processed',
])

const PLANNED = new Set<RequestStatus>(['draft', 'pending_manager', 'pending_hr'])

export type BucketName = 'planned' | 'approved' | 'moved' | 'closed'

export function isMoved(request: LeaveRequest, all: readonly LeaveRequest[]): boolean {
  if (request.replacesRequestId || request.replacedByRequestId) return true
  return all.some(
    (other) => other.replacesRequestId === request.id || other.replacedByRequestId === request.id,
  )
}

export function bucketOf(request: LeaveRequest, all: readonly LeaveRequest[]): BucketName {
  const moved = isMoved(request, all)
  if (request.status === 'rejected' || request.status === 'cancelled') return moved ? 'moved' : 'closed'
  if (moved) return 'moved'
  if (APPROVED.has(request.status)) return 'approved'
  if (PLANNED.has(request.status)) return 'planned'
  return 'closed'
}

export type BalanceSnapshot = {
  freeDays: number
  hasStreak: boolean
  withinFirstSixMonths: boolean
  unpaidDayOffDays: number
}

export type PreviewIssue = { code: string; level: 'reject' | 'warn' }

export type LeavePreview = CountedRange & {
  payBy: string
  payByClassic: string
  issues: PreviewIssue[]
}

type ExistingRange = {
  id: string
  type: RequestType
  status: RequestStatus
  start: string
  end: string
}

export function previewLeave(input: {
  type: RequestType
  start: string
  end: string
  reasonCode?: DayOffReason
  calendar?: ReadonlyMap<string, DayKind>
  existing?: readonly ExistingRange[]
  ignoreIds?: readonly string[]
  balance?: BalanceSnapshot | null
  today: string
}): LeavePreview {
  const counted = countRange(input.start, input.end, input.calendar)
  const pay = payByDates(input.start || input.today)
  const issues: PreviewIssue[] = []
  if (!input.start || !input.end || input.end < input.start) {
    issues.push({ code: 'validation', level: 'reject' })
  } else if (counted.countedDays === 0) {
    issues.push({ code: 'empty_range', level: 'reject' })
  }

  const ignore = new Set(input.ignoreIds ?? [])
  for (const other of input.existing ?? []) {
    if (ignore.has(other.id)) continue
    if (other.status === 'rejected' || other.status === 'cancelled' || other.status === 'draft') continue
    if (!rangesOverlap(input.start, input.end, other.start, other.end)) continue
    if (input.type === 'sick' && other.type === 'vacation') {
      const days = countedOverlap(input.start, input.end, other.start, other.end, input.calendar)
      if (days > 0 && !issues.some((issue) => issue.code === 'sick_overlap')) {
        issues.push({ code: 'sick_overlap', level: 'warn' })
      }
      continue
    }
    issues.push({ code: 'overlap', level: 'reject' })
    break
  }

  if (input.type === 'vacation' && input.balance && counted.countedDays > 0) {
    if (input.balance.withinFirstSixMonths && counted.countedDays > input.balance.freeDays) {
      issues.push({ code: 'balance_exceeded', level: 'reject' })
    } else if (counted.countedDays > input.balance.freeDays) {
      issues.push({ code: 'advance_days', level: 'warn' })
    }
    const remaining = input.balance.freeDays - counted.countedDays
    if (!input.balance.hasStreak && counted.countedDays < 14 && remaining < 14) {
      issues.push({ code: 'streak_impossible', level: 'reject' })
    }
  }

  if (input.type === 'vacation' && input.start && pay.stricter < input.today) {
    issues.push({ code: 'pay_by_warning', level: 'warn' })
  }

  if (
    input.type === 'day_off' &&
    input.reasonCode &&
    UNPAID_REASONS.has(input.reasonCode) &&
    input.balance &&
    input.balance.unpaidDayOffDays + counted.calendarDays > 14
  ) {
    issues.push({ code: 'unpaid_day_off_excess', level: 'warn' })
  }

  return {
    ...counted,
    payBy: pay.stricter,
    payByClassic: pay.classic,
    issues,
  }
}

export function hasReject(preview: LeavePreview): boolean {
  return preview.issues.some((issue) => issue.level === 'reject')
}

export type ConfirmationState = 'ready' | 'upcoming' | 'passed' | 'none'

export function dayOffHasLeadTime(request: Pick<LeaveRequest, 'type' | 'createdAt' | 'start'>): boolean {
  if (request.type !== 'day_off') return false
  return daysUntil(request.createdAt.slice(0, 10), request.start) >= 5
}

export function confirmationState(request: LeaveRequest, today: string): ConfirmationState {
  if (request.type === 'sick') return 'none'
  if (['rejected', 'cancelled', 'confirmed', 'processed', 'draft'].includes(request.status)) return 'none'
  if (request.status !== 'scheduled' && request.status !== 'awaiting_confirmation') return 'none'
  if (request.type === 'day_off' && request.status !== 'awaiting_confirmation' && !dayOffHasLeadTime(request)) {
    return 'none'
  }
  const days = daysUntil(today, request.start)
  if (days < 0) return 'none'
  if (request.status === 'awaiting_confirmation') return 'ready'
  if (days > 5) return 'upcoming'
  if (days >= 3) return 'ready'
  return 'passed'
}

export function noticeDue(request: LeaveRequest, today: string): boolean {
  if (request.type !== 'vacation' || request.acknowledgedAt) return false
  if (!['scheduled', 'awaiting_confirmation', 'confirmed'].includes(request.status)) return false
  const days = daysUntil(today, request.start)
  return days >= 0 && days <= 14
}

export function canCancel(request: LeaveRequest): boolean {
  return !['processed', 'rejected', 'cancelled'].includes(request.status)
}

export function canReschedule(request: LeaveRequest): boolean {
  return !['rejected', 'cancelled', 'draft'].includes(request.status)
}

export type TimelineStep = 'manager' | 'hr' | 'scheduled' | 'confirmation' | 'processed'

export function timelineSteps(request: LeaveRequest): TimelineStep[] {
  const steps: TimelineStep[] = ['manager', 'hr', 'scheduled']
  const needsConfirmation =
    request.type === 'vacation' ||
    request.type === 'day_off' && (dayOffHasLeadTime(request) || request.status === 'awaiting_confirmation' || request.status === 'confirmed')
  if (request.type !== 'sick' && needsConfirmation) steps.push('confirmation')
  if (request.type === 'vacation') steps.push('processed')
  return steps
}

export function timelineIndex(request: LeaveRequest): number {
  const steps = timelineSteps(request)
  const statusToStep: Partial<Record<RequestStatus, TimelineStep>> = {
    pending_manager: 'manager',
    pending_hr: 'hr',
    scheduled: 'scheduled',
    awaiting_confirmation: 'confirmation',
    confirmed: steps.includes('confirmation') ? 'confirmation' : 'scheduled',
    processed: 'processed',
  }
  const step = statusToStep[request.status]
  const index = step ? steps.indexOf(step) : -1
  if (request.status === 'confirmed' && steps.includes('processed')) return steps.indexOf('confirmation')
  return index
}
