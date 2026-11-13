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
    getMe: () => readJson<Me>(paths.me()),
    updateLanguage: (language: Language) =>
      readJson<Me>(paths.me(), { method: 'PATCH', body: JSON.stringify({ language }) }),
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
  }
}
