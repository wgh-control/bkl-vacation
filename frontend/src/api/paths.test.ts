import { describe, expect, it } from 'vitest'
import { paths } from './paths'

describe('paths', () => {
  it('builds the plan routes and the report query', () => {
    expect(paths.me()).toBe('/me')
    expect(paths.balance('2026-11-13')).toBe('/balance?on=2026-11-13')
    expect(paths.confirm('a b')).toBe('/requests/a%20b/confirm')
    expect(paths.decision('step-1')).toBe('/approvals/step-1/decision')
    expect(paths.processed('req-1')).toBe('/hr/requests/req-1/processed')
    expect(
      paths.export({
        format: 'pdf',
        scope: 'department',
        departmentId: 'dept-people',
        from: '2026-01-01',
        to: '2026-12-31',
      }),
    ).toBe('/hr/export?format=pdf&scope=department&departmentId=dept-people&from=2026-01-01&to=2026-12-31')
    expect(paths.export({ format: 'xls', scope: 'all' })).toBe('/hr/export?format=xls&scope=all')
    expect(paths.export({ format: 'xls', scope: 'person', personId: 'emp-ivan' })).toBe(
      '/hr/export?format=xls&scope=person&personId=emp-ivan',
    )
  })
})
