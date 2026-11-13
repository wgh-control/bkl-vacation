export class ApiError extends Error {
  readonly code: string
  readonly status: number

  constructor(code: string, status: number) {
    super(code)
    this.name = 'ApiError'
    this.code = code
    this.status = status
  }
}

export function normalizeCode(code: unknown): string {
  if (typeof code !== 'string' || !code.trim()) return 'unknown'
  return code.trim().toLowerCase().replace(/[\s-]+/g, '_')
}

export function asList<T>(body: unknown): T[] {
  if (Array.isArray(body)) return body as T[]
  if (body && typeof body === 'object' && Array.isArray((body as { items?: unknown }).items)) {
    return (body as { items: T[] }).items
  }
  throw new ApiError('validation', 200)
}
