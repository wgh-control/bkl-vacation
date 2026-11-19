import { ApiError, asList, normalizeCode } from './errors'
import { paths } from './paths'
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
  ManagedPerson,
  ManagerDirectory,
  Me,
  OverlapChoice,
  PersonRef,
  ReportFile,
  RequestQuery,
  VacationApi,
} from './types'

/**
 * Thin client for the Go API. Screens talk to `VacationApi`.
 * When an OpenAPI document arrives, replace this file and keep the interface.
 */

type HttpOptions = {
  baseUrl?: string
  token: () => Promise<string>
}

function filenameFrom(header: string | null, fallback: string): string {
  if (!header) return fallback
  const encoded = /filename\*=UTF-8''([^;]+)/i.exec(header)
  if (encoded) return decodeURIComponent(encoded[1])
  const plain = /filename="?([^";]+)"?/i.exec(header)
  return plain ? plain[1] : fallback
}

export function createHttpApi(options: HttpOptions): VacationApi {
  const base = (options.baseUrl || import.meta.env.VITE_API_BASE || '/api').replace(/\/$/, '')

  async function send(path: string, init: RequestInit = {}): Promise<Response> {
    const token = await options.token()
    const headers = new Headers(init.headers)
    headers.set('Authorization', `Bearer ${token}`)
    if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
    const response = await fetch(`${base}${path}`, { ...init, headers })
    if (!response.ok) {
      let code = 'unknown'
      try {
        const body: unknown = await response.json()
        if (body && typeof body === 'object') {
          const record = body as { code?: unknown; error?: unknown }
          code = normalizeCode(record.code ?? record.error)
        }
      } catch {
        if (response.status === 401) code = 'unauthorized'
        else if (response.status === 403) code = 'forbidden'
        else if (response.status === 404) code = 'not_found'
        else if (response.status === 409) code = 'conflict'
      }
      throw new ApiError(code, response.status)
    }
    return response
  }

  async function readJson<T>(path: string, init?: RequestInit): Promise<T> {
    const response = await send(path, init)
    return (await response.json()) as T
  }

  async function readList<T>(path: string): Promise<T[]> {
    return asList<T>(await readJson<unknown>(path))
  }

  function post(path: string, body?: unknown): Promise<LeaveRequest> {
    return readJson<LeaveRequest>(path, {
      method: 'POST',
      body: body === undefined ? '{}' : JSON.stringify(body),
    })
  }

  return {
    getMe: async () => asMe(await readJson<unknown>(paths.me())),
    updateLanguage: async (language: Language) =>
      asMe(await readJson<unknown>(paths.me(), { method: 'PATCH', body: JSON.stringify({ locale: language }) })),
    getBalance: (on: string) => readJson<Balance>(paths.balance(on)),
    getCalendar: (from: string, to: string) => readList<CalendarDay>(paths.calendar(from, to)),
    getRequests: (query?: RequestQuery) => readList<LeaveRequest>(paths.requests(query)),
    getRequest: (requestId: string) => readJson<LeaveRequest>(paths.request(requestId)),
    createRequest: (body: CreateRequestBody) => post(paths.requests(), body),
    cancelRequest: (requestId: string) => post(paths.cancel(requestId)),
    rescheduleRequest: (requestId, body) => post(paths.reschedule(requestId), body),
    confirmRequest: (requestId: string) => post(paths.confirm(requestId)),
    acknowledgeRequest: (requestId: string) => post(paths.acknowledge(requestId)),
    setCertificate: (requestId, certificateNumber) => post(paths.certificate(requestId), { certificateNumber }),
    chooseOverlap: (requestId: string, choice: OverlapChoice) => post(paths.overlap(requestId), { choice }),
    getApprovals: () => readList<ApprovalItem>(paths.approvals()),
    decide: (stepId, body) => post(paths.decision(stepId), body),
    getHrQueue: () => readJson<HrQueue>(paths.hrQueue()),
    markProcessed: (requestId: string) => post(paths.processed(requestId)),
    async exportReport(query: ExportQuery): Promise<ReportFile> {
      const response = await send(paths.export(query))
      const blob = await response.blob()
      const extension = query.format === 'pdf' ? 'pdf' : 'xls'
      const fallback = `bkl-leave-${query.scope}.${extension}`
      return { blob, filename: filenameFrom(response.headers.get('Content-Disposition'), fallback) }
    },
    listPeople: (q?: string) => readList<PersonRef>(paths.people(q)),
    listDepartments: () => readList<Department>(paths.departments()),
    getManagerDirectory: async () => mapDirectory(await readJson<unknown>(paths.managers())),
    setManager: async (employeeId, body) =>
      mapDirectory(
        await readJson<unknown>(paths.manager(employeeId), {
          method: 'POST',
          body: JSON.stringify({ action: body.action, manager_id: body.managerId ?? '' }),
        }),
      ),
  }
}

function asMe(raw: unknown): Me {
  if (!raw || typeof raw !== 'object') throw new ApiError('unknown', 500)
  const record = raw as Record<string, unknown>
  if (typeof record.displayName === 'string' && Array.isArray(record.roles)) return raw as Me
  const department = record.department
  let departmentName: string | undefined
  let departmentId: string | undefined
  if (department && typeof department === 'object') {
    const dept = department as { id?: unknown; name?: unknown }
    if (typeof dept.name === 'string' && dept.name) departmentName = dept.name
    if (typeof dept.id === 'string' && dept.id) departmentId = dept.id
  }
  const roles: Me['roles'] = ['employee']
  if (record.manager_of_others === true) roles.push('manager')
  if (record.hr === true) roles.push('hr')
  return {
    id: String(record.id ?? ''),
    subject: String(record.subject ?? ''),
    displayName: String(record.display_name ?? record.displayName ?? ''),
    email: String(record.email ?? ''),
    language: record.locale === 'en' || record.language === 'en' ? 'en' : 'ru',
    employeeNumber: textOrUndefined(record.employee_number ?? record.employeeNumber),
    position: textOrUndefined(record.position),
    department: departmentName ?? textOrUndefined(department),
    departmentId,
    hireDate: textOrUndefined(record.hire_date ?? record.hireDate),
    manager: asPerson(record.manager),
    roles,
    active: record.active !== false,
  }
}

function mapDirectory(raw: unknown): ManagerDirectory {
  const body = raw && typeof raw === 'object' ? (raw as { groups?: unknown }) : {}
  const groups = Array.isArray(body.groups) ? body.groups : []
  return { groups: groups.map(mapGroup) }
}

function mapGroup(value: unknown): ManagerDirectory['groups'][number] {
  const group = value && typeof value === 'object' ? (value as { manager?: unknown; people?: unknown }) : {}
  const people = Array.isArray(group.people) ? group.people : []
  return { manager: asPerson(group.manager), people: people.map(mapManaged) }
}

function mapManaged(value: unknown): ManagedPerson {
  const person = value && typeof value === 'object' ? (value as Record<string, unknown>) : {}
  if (typeof person.displayName === 'string' && 'manual' in person && 'adManager' in person) {
    return value as ManagedPerson
  }
  return {
    id: String(person.id ?? ''),
    displayName: String(person.name ?? person.display_name ?? person.displayName ?? ''),
    email: textOrUndefined(person.email),
    employeeNumber: textOrUndefined(person.employee_number ?? person.employeeNumber),
    position: textOrUndefined(person.position),
    department: textOrUndefined(person.department),
    active: person.active !== false,
    manual: person.manual === true,
    manager: asPerson(person.manager),
    adManager: asPerson(person.ad_manager ?? person.adManager),
    adManagerRaw: textOrUndefined(person.ad_manager_raw ?? person.adManagerRaw),
    override: asPerson(person.override),
    adChange: asPerson(person.ad_change ?? person.adChange),
    adChangeRaw: textOrUndefined(person.ad_change_raw ?? person.adChangeRaw),
  }
}

function asPerson(value: unknown): PersonRef | null {
  if (!value || typeof value !== 'object') return null
  const ref = value as { id?: unknown; name?: unknown; displayName?: unknown; display_name?: unknown }
  const id = typeof ref.id === 'string' ? ref.id : ''
  const displayName =
    typeof ref.displayName === 'string'
      ? ref.displayName
      : typeof ref.display_name === 'string'
        ? ref.display_name
        : typeof ref.name === 'string'
          ? ref.name
          : ''
  if (!id && !displayName) return null
  return { id, displayName }
}

function textOrUndefined(value: unknown): string | undefined {
  return typeof value === 'string' && value ? value : undefined
}
