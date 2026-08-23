import type {
  ErrorResponse,
  SessionResponse,
  MetaResponse,
  OverviewResponse,
  RankingsResponse,
  ConnectionPage,
  DimensionSeriesResponse,
} from './types'

export class ApiError extends Error {
  code: string
  requestId: string
  retryable: boolean
  status: number
  details?: Record<string, unknown>

  constructor(status: number, errorBody: ErrorResponse['error']) {
    super(errorBody.message)
    this.name = 'ApiError'
    this.status = status
    this.code = errorBody.code
    this.requestId = errorBody.requestId
    this.retryable = errorBody.retryable
    this.details = errorBody.details
  }
}

const BASE_URL = '/api/v1'

async function handleResponse<T>(res: Response): Promise<T> {
  if (res.status === 204) return undefined as T
  const contentType = res.headers.get('content-type')
  if (contentType?.includes('application/json')) {
    let data: unknown
    try {
      data = await res.json()
    } catch {
      throw new ApiError(res.status, {
        code: 'INTERNAL_ERROR',
        message: 'The server returned malformed JSON.',
        requestId: res.headers.get('x-request-id') ?? '',
        retryable: res.status >= 500,
      })
    }
    if (!res.ok) {
      const body = data as Partial<ErrorResponse>
      if (body.error && typeof body.error.code === 'string' && typeof body.error.message === 'string') {
        throw new ApiError(res.status, body.error)
      }
      throw new ApiError(res.status, {
        code: res.status === 401 ? 'UNAUTHORIZED' : 'INTERNAL_ERROR',
        message: res.statusText || 'Request failed.',
        requestId: res.headers.get('x-request-id') ?? '',
        retryable: res.status >= 500,
      })
    }
    return data as T
  }
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new ApiError(res.status, {
      code: 'INTERNAL_ERROR',
      message: text || res.statusText,
      requestId: '',
      retryable: res.status >= 500,
    })
  }
  return undefined as T
}

export interface FetchOptions extends RequestInit {
  params?: Record<string, string | number | boolean | undefined>
}

function buildUrl(path: string, params?: Record<string, string | number | boolean | undefined>): string {
  const url = new URL(path, window.location.origin)
  if (params) {
    Object.entries(params).forEach(([k, v]) => {
      if (v !== undefined) url.searchParams.set(k, String(v))
    })
  }
  return url.pathname + url.search
}

export async function apiFetch<T>(path: string, options: FetchOptions = {}): Promise<T> {
  const { params, ...init } = options
  const url = buildUrl(`${BASE_URL}${path}`, params)
  const res = await fetch(url, {
    ...init,
    credentials: 'same-origin',
    headers: {
      'Content-Type': 'application/json',
      ...(init.headers || {}),
    },
  })
  try {
    return await handleResponse<T>(res)
  } catch (error) {
    if (error instanceof ApiError && error.status === 401 && path !== '/session') {
      window.dispatchEvent(new Event('observability:session-expired'))
    }
    throw error
  }
}

export const api = {
  getHealth: () => fetch('/healthz', { credentials: 'same-origin' }).then(r => handleResponse<{ status: string }>(r)),
  getReady: () => fetch('/readyz', { credentials: 'same-origin' }).then(r => handleResponse<{ status: string }>(r)),

  getSession: () => apiFetch<SessionResponse>('/session'),
  login: (token: string) => apiFetch<SessionResponse>('/session', { method: 'POST', body: JSON.stringify({ token }) }),
  logout: () => apiFetch<void>('/session', { method: 'DELETE' }),

  getMeta: () => apiFetch<MetaResponse>('/meta'),
  getOverview: (params?: { range?: string; from?: string; to?: string; step?: string }) =>
    apiFetch<OverviewResponse>('/overview', { params }),
  getRankings: (params: {
    dimension: string
    sort?: string
    limit?: number
    range?: string
    from?: string
    to?: string
  }) => apiFetch<RankingsResponse>('/rankings', { params }),
  getDimensionSeries: (params: {
    dimension: 'network' | 'inbound' | 'outbound'
    value: string
    range?: string
    from?: string
    to?: string
    step?: string
  }) => apiFetch<DimensionSeriesResponse>('/dimensions/series', { params }),
  getActiveConnections: (params?: {
    q?: string
    network?: string
    outbound?: string
    limit?: number
    offset?: number
  }) => apiFetch<ConnectionPage>('/connections/active', { params }),
  getRecentConnections: (params?: {
    range?: string
    from?: string
    to?: string
    q?: string
    network?: string
    outbound?: string
    limit?: number
    offset?: number
  }) => apiFetch<ConnectionPage>('/connections/recent', { params }),
}

export const SSE_URL = '/api/v1/events'
