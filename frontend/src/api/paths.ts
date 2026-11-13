import type { ExportQuery, RequestQuery } from './types'

/**
 * Paths the portal calls. The plan's surface is under /api.
 * Extra paths the screens need, until an OpenAPI document replaces this file:
 * GET /requests/{id}
 * POST /requests/{id}/acknowledge
 * POST /requests/{id}/certificate
 * POST /requests/{id}/overlap
 * GET /hr/people
 * GET /hr/departments
 * Query on GET /balance, /calendar, /requests, and /hr/export.
 */

function query(path: string, params: Record<string, string | undefined>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value) search.set(key, value)
  }
  const text = search.toString()
  return text ? `${path}?${text}` : path
}

function id(value: string): string {
  return encodeURIComponent(value)
}

export const paths = {
  me: () => '/me',
  balance: (on: string) => query('/balance', { on }),
  calendar: (from: string, to: string) => query('/calendar', { from, to }),
  requests: (params: RequestQuery = {}) => query('/requests', params),
  request: (requestId: string) => `/requests/${id(requestId)}`,
  cancel: (requestId: string) => `/requests/${id(requestId)}/cancel`,
  reschedule: (requestId: string) => `/requests/${id(requestId)}/reschedule`,
  confirm: (requestId: string) => `/requests/${id(requestId)}/confirm`,
  acknowledge: (requestId: string) => `/requests/${id(requestId)}/acknowledge`,
  certificate: (requestId: string) => `/requests/${id(requestId)}/certificate`,
  overlap: (requestId: string) => `/requests/${id(requestId)}/overlap`,
  approvals: () => '/approvals',
  decision: (stepId: string) => `/approvals/${id(stepId)}/decision`,
  hrQueue: () => '/hr/queue',
  processed: (requestId: string) => `/hr/requests/${id(requestId)}/processed`,
  export: (params: ExportQuery) =>
    query('/hr/export', {
      format: params.format,
      scope: params.scope,
      personId: params.personId,
      departmentId: params.departmentId,
      from: params.from,
      to: params.to,
    }),
  people: (q?: string) => query('/hr/people', { q }),
  departments: () => '/hr/departments',
}
