import { describe, expect, it } from 'vitest'
import type { LeaveRequest } from '../api/types'
import { bucketOf, confirmationState, previewLeave } from './leave'

function request(partial: Partial<LeaveRequest> & Pick<LeaveRequest, 'id' | 'status' | 'start' | 'end'>): LeaveRequest {
  return {
    type: 'vacation',
    person: { id: 'p', displayName: 'Person' },
    countedDays: 1,
    createdAt: '2026-01-01T00:00:00',
    ...partial,
  }
}

describe('bucketOf', () => {
  it('splits planned, approved, and moved days', () => {
    const planned = request({ id: 'p', status: 'pending_manager', start: '2026-12-01', end: '2026-12-02' })
    const approved = request({ id: 'a', status: 'scheduled', start: '2026-08-01', end: '2026-08-14' })
    const original = request({
      id: 'o',
      status: 'scheduled',
      start: '2026-05-01',
      end: '2026-05-05',
      replacedByRequestId: 'n',
    })
    const successor = request({
      id: 'n',
      status: 'pending_hr',
      start: '2026-09-01',
      end: '2026-09-05',
      replacesRequestId: 'o',
    })
    const all = [planned, approved, original, successor]
    expect(bucketOf(planned, all)).toBe('planned')
    expect(bucketOf(approved, all)).toBe('approved')
    expect(bucketOf(original, all)).toBe('moved')
    expect(bucketOf(successor, all)).toBe('moved')
  })
})

describe('previewLeave', () => {
  it('rejects a vacation that would make the 14-day part impossible', () => {
    const preview = previewLeave({
      type: 'vacation',
      start: '2026-12-01',
      end: '2026-12-05',
      today: '2026-11-13',
      balance: { freeDays: 14, hasStreak: false, withinFirstSixMonths: false, unpaidDayOffDays: 0 },
    })
    expect(preview.issues.map((issue) => issue.code)).toContain('streak_impossible')
  })

  it('allows a sick day that overlaps a vacation and warns', () => {
    const preview = previewLeave({
      type: 'sick',
      start: '2026-08-03',
      end: '2026-08-04',
      today: '2026-08-01',
      existing: [{ id: 'v', type: 'vacation', status: 'scheduled', start: '2026-08-01', end: '2026-08-14' }],
    })
    expect(preview.issues).toEqual([{ code: 'sick_overlap', level: 'warn' }])
  })
})

describe('confirmationState', () => {
  const base = request({
    id: 'c',
    status: 'scheduled',
    start: '2026-11-17',
    end: '2026-11-20',
    createdAt: '2026-10-01T00:00:00',
  })

  it('is ready inside the 3 to 5 day window', () => {
    expect(confirmationState(base, '2026-11-13')).toBe('ready')
    expect(confirmationState({ ...base, start: '2026-11-16' }, '2026-11-13')).toBe('ready')
  })

  it('stays upcoming before the window and does not cancel after it', () => {
    expect(confirmationState({ ...base, start: '2026-11-30' }, '2026-11-13')).toBe('upcoming')
    expect(confirmationState({ ...base, start: '2026-11-15' }, '2026-11-13')).toBe('passed')
  })

  it('skips sick days and a day off requested inside five days', () => {
    expect(confirmationState({ ...base, type: 'sick' }, '2026-11-13')).toBe('none')
    expect(
      confirmationState(
        { ...base, type: 'day_off', createdAt: '2026-11-13T00:00:00', start: '2026-11-16' },
        '2026-11-13',
      ),
    ).toBe('none')
  })
})
