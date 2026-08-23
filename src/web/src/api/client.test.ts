import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import { server } from '@/mocks/node'
import { api, apiFetch } from './client'

describe('API client', () => {
  it('preserves the structured error envelope and request id', async () => {
    server.use(http.get('/api/v1/meta', () => HttpResponse.json({
      error: {
        code: 'STORAGE_UNAVAILABLE',
        message: 'storage unavailable',
        requestId: 'req_test',
        retryable: true,
        details: { shard: 'local' },
      },
    }, { status: 503 })))

    const promise = api.getMeta()
    await expect(promise).rejects.toMatchObject({
      name: 'ApiError',
      status: 503,
      code: 'STORAGE_UNAVAILABLE',
      requestId: 'req_test',
      retryable: true,
    })
  })

  it('handles 204 without attempting to decode JSON', async () => {
    await expect(api.logout()).resolves.toBeUndefined()
  })

  it('turns malformed JSON into a stable ApiError', async () => {
    server.use(http.get('/api/v1/broken', () => new HttpResponse('{', {
      status: 500,
      headers: { 'Content-Type': 'application/json', 'X-Request-ID': 'req_broken' },
    })))

    await expect(apiFetch('/broken')).rejects.toMatchObject({
      code: 'INTERNAL_ERROR',
      requestId: 'req_broken',
      retryable: true,
    })
  })

  it('notifies the router when a protected request loses its session', async () => {
    server.use(http.get('/api/v1/meta', () => HttpResponse.json({
      error: { code: 'UNAUTHORIZED', message: 'expired', requestId: 'req_401', retryable: false },
    }, { status: 401 })))
    const listener = vi.fn()
    window.addEventListener('observability:session-expired', listener)

    await expect(api.getMeta()).rejects.toMatchObject({ status: 401 })
    expect(listener).toHaveBeenCalledOnce()
    window.removeEventListener('observability:session-expired', listener)
  })
})
