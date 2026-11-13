import { describe, expect, it } from 'vitest'
import { bucketOf } from '../domain/leave'
import { createFixtureApi } from './fixture'

const TODAY = '2026-11-13'

describe('fixture workflow', () => {
  it('keeps an employee plan of planned, approved, and moved days across years', async () => {
    const api = createFixtureApi({ today: TODAY })
    const requests = await api.getRequests()
    const inYear = (year: number) => requests.filter((request) => request.start.slice(0, 4) === String(year) || request.end.slice(0, 4) === String(year))
    const buckets = (year: number) => inYear(year).map((request) => bucketOf(request, requests))
    expect(buckets(2025)).toContain('approved')
    expect(buckets(2026)).toEqual(expect.arrayContaining(['planned', 'approved', 'moved']))
    expect(buckets(2027)).toContain('moved')

    const balance = await api.getBalance(TODAY)
    expect(balance.grantedDays).toBe(28)
    expect(balance.hasStreak).toBe(false)
    expect(balance.streakPossible).toBe(true)
    expect(balance.freeDays).toBeGreaterThanOrEqual(14)
  })

  it('runs manager approval, then HR, then confirmation and processed', async () => {
    const api = createFixtureApi({ today: TODAY })
    const approvals = await api.getApprovals()
    const ivan = approvals.find((item) => item.request.person.id === 'emp-ivan' && item.role === 'manager')
    expect(ivan).toBeTruthy()
    const afterManager = await api.decide(ivan!.id, { decision: 'approve', comment: 'ok' })
    expect(afterManager.status).toBe('pending_hr')

    const hr = (await api.getApprovals()).find((item) => item.request.id === afterManager.id)
    const scheduled = await api.decide(hr!.id, { decision: 'approve' })
    expect(scheduled.status).toBe('scheduled')

    const mine = await api.getRequest('req-confirm')
    const confirmed = await api.confirmRequest(mine.id)
    expect(confirmed.status).toBe('confirmed')

    const queue = await api.getHrQueue()
    expect(queue.awaitingProcessed.some((request) => request.id === confirmed.id)).toBe(true)
    const processed = await api.markProcessed(confirmed.id)
    expect(processed.status).toBe('processed')
    expect(processed.processedBy?.id).toBe('emp-anna')
  })

  it('approves sick days and day offs on the same chain and blocks a sick day without a certificate', async () => {
    const api = createFixtureApi({ today: TODAY })
    const queue = await api.getHrQueue()
    const sick = queue.pendingApproval.find((request) => request.id === 'req-ivan-sick')
    const dayOff = queue.pendingApproval.find((request) => request.id === 'req-ivan-dayoff')
    const missing = queue.pendingApproval.find((request) => request.id === 'req-maria-sick')
    expect(sick?.type).toBe('sick')
    expect(dayOff?.type).toBe('day_off')
    await expect(api.decide(missing!.pendingStepId!, { decision: 'approve' })).rejects.toMatchObject({
      code: 'certificate_required',
    })
    expect((await api.decide(sick!.pendingStepId!, { decision: 'approve' })).status).toBe('scheduled')
    expect((await api.decide(dayOff!.pendingStepId!, { decision: 'approve' })).status).toBe('scheduled')
  })

  it('reschedules through manager and HR and keeps the old dates until the new ones are approved', async () => {
    const api = createFixtureApi({ today: TODAY })
    const successor = await api.rescheduleRequest('req-processed-ready', {
      start: '2027-07-01',
      end: '2027-07-14',
    })
    expect(successor.status).toBe('pending_manager')
    expect(successor.replacesRequestId).toBe('req-processed-ready')
    expect((await api.getRequest('req-processed-ready')).status).toBe('confirmed')

    api.setCurrentUser('emp-oleg')
    const step = (await api.getApprovals()).find((item) => item.request.id === successor.id)
    expect(step?.role).toBe('manager')
    const pendingHr = await api.decide(step!.id, { decision: 'approve' })
    expect(pendingHr.status).toBe('pending_hr')

    api.setCurrentUser('emp-anna')
    const hrStep = (await api.getApprovals()).find((item) => item.request.id === successor.id)
    const approved = await api.decide(hrStep!.id, { decision: 'approve' })
    expect(['scheduled', 'awaiting_confirmation']).toContain(approved.status)
    expect((await api.getRequest('req-processed-ready')).status).toBe('cancelled')
  })

  it('downloads pdf and xls for one person, a department, and everyone', async () => {
    const api = createFixtureApi({ today: TODAY })
    const period = { from: '2025-01-01', to: '2027-12-31' }
    const person = await api.exportReport({ format: 'pdf', scope: 'person', personId: 'emp-ivan', ...period })
    const department = await api.exportReport({
      format: 'xls',
      scope: 'department',
      departmentId: 'dept-finance',
      ...period,
    })
    const everyone = await api.exportReport({ format: 'xls', scope: 'all', ...period })
    expect(person.filename.endsWith('.pdf')).toBe(true)
    expect(department.filename.endsWith('.xls')).toBe(true)
    const read = (blob: Blob) =>
      new Promise<string>((resolve, reject) => {
        const reader = new FileReader()
        reader.onload = () => resolve(String(reader.result))
        reader.onerror = () => reject(reader.error)
        reader.readAsText(blob)
      })
    expect(await read(person.blob)).toContain('%PDF')
    const sheet = await read(everyone.blob)
    expect(sheet).toContain('Соколова Анна')
    expect(sheet).toContain('Морозова Мария')
    const finance = await read(department.blob)
    expect(finance).toContain('Морозова Мария')
    expect(finance).not.toContain('Иванов Пётр')

    api.setRoles(['employee'])
    await expect(api.exportReport({ format: 'pdf', scope: 'all' })).rejects.toMatchObject({ code: 'forbidden' })
    const me = await api.getMe()
    expect(me.roles).toContain('employee')
    expect(me.roles).not.toContain('hr')
  })

  it('plans a vacation in the next working year', async () => {
    const api = createFixtureApi({ today: TODAY })
    const created = await api.createRequest({ type: 'vacation', start: '2027-07-01', end: '2027-07-14' })
    expect(created.status).toBe('pending_manager')
    expect(created.countedDays).toBeGreaterThanOrEqual(14)
    const next = await api.getRequests({ from: '2027-01-01', to: '2027-12-31' })
    expect(next.some((request) => request.id === created.id)).toBe(true)
  })
})
