import { describe, expect, it } from 'vitest'
import { countRange, payByDates, workingYearContaining } from './dates'

describe('countRange', () => {
  it('counts weekends and skips federal holidays', () => {
    const counted = countRange('2026-02-21', '2026-02-23')
    expect(counted.calendarDays).toBe(3)
    expect(counted.holidayDates).toEqual(['2026-02-23'])
    expect(counted.weekendDates).toEqual(['2026-02-21', '2026-02-22'])
    expect(counted.countedDays).toBe(2)
  })

  it('rejects an empty counted range when every day is a holiday', () => {
    expect(countRange('2026-01-01', '2026-01-08').countedDays).toBe(0)
  })
})

describe('payByDates', () => {
  it('uses the stricter date that leaves the payment day outside the three days', () => {
    expect(payByDates('2026-06-10')).toEqual({ classic: '2026-06-07', stricter: '2026-06-06' })
  })
})

describe('workingYearContaining', () => {
  it('starts on the hire anniversary', () => {
    expect(workingYearContaining('2019-04-15', '2026-11-13')).toEqual({
      start: '2026-04-15',
      end: '2027-04-14',
    })
    expect(workingYearContaining('2019-04-15', '2026-04-01')).toEqual({
      start: '2025-04-15',
      end: '2026-04-14',
    })
  })
})
