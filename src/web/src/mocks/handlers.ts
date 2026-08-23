import { http, HttpResponse } from 'msw'
import type { Connection, MetaResponse, OverviewResponse, SeriesPoint, RankingEntry, SessionResponse } from '@/api/types'

const baseTime = new Date('2026-08-21T00:00:00Z').getTime()

function point(ts: number): SeriesPoint {
  return {
    timestamp: new Date(ts).toISOString(),
    uploadBytesPerSecond: Math.floor(15000 + Math.random() * 20000),
    downloadBytesPerSecond: Math.floor(500000 + Math.random() * 800000),
    activeConnections: Math.floor(8 + Math.random() * 15),
    memoryBytes: 70000000 + Math.floor(Math.random() * 10000000),
    goroutines: 80 + Math.floor(Math.random() * 20),
  }
}

function generateSeries(count: number, stepSec: number): SeriesPoint[] {
  const out: SeriesPoint[] = []
  for (let i = count - 1; i >= 0; i--) {
    out.push(point(baseTime - i * stepSec * 1000))
  }
  return out
}

const metaOnline: MetaResponse = {
  apiVersion: 'v1',
  appVersion: '0.1.0-dev',
  generatedAt: new Date(baseTime).toISOString(),
  source: {
    state: 'online',
    displayName: 'local',
    lastAttemptAt: new Date(baseTime).toISOString(),
    lastSuccessAt: new Date(baseTime).toISOString(),
    lastErrorCode: null,
    historyAvailableFrom: new Date(baseTime - 7 * 24 * 3600 * 1000).toISOString(),
  },
  capabilities: {
    upstreamApiVersion: 1,
    endpoints: ['capabilities', 'metrics', 'status', 'connections/active', 'connections/recent', 'events', 'top'],
    exposeSensitive: true,
    rankingDimensions: ['network', 'inbound', 'outbound', 'rule', 'domain', 'destination_ip', 'source', 'process', 'user'],
    sensitiveDimensions: ['rule', 'domain', 'destination_ip', 'source', 'process', 'user'],
    recentConnectionLimit: 2000,
    recentTtlSeconds: 3600,
    topKSize: 100,
    activePageLimit: 500,
    cursorPagination: true,
    eventReplay: false,
  },
  collector: {
    scrapeIntervalSeconds: 2,
    persistIntervalSeconds: 15,
    retentionSeconds: 604800,
    maxSeriesPoints: 720,
    channels: {
      capabilities: { state: 'online', lastAttemptAt: new Date(baseTime).toISOString(), lastSuccessAt: new Date(baseTime).toISOString(), lastErrorCode: null },
      metrics: { state: 'online', lastAttemptAt: new Date(baseTime).toISOString(), lastSuccessAt: new Date(baseTime).toISOString(), lastErrorCode: null },
      connections: { state: 'online', lastAttemptAt: new Date(baseTime).toISOString(), lastSuccessAt: new Date(baseTime).toISOString(), lastErrorCode: null },
      events: { state: 'online', lastAttemptAt: new Date(baseTime).toISOString(), lastSuccessAt: new Date(baseTime).toISOString(), lastErrorCode: null, reconnects: 0, lastEventAt: new Date(baseTime).toISOString(), lastSequence: 42 },
    },
  },
}

const outboundNames = [
  'Proxy > HK - CMI Premium Node 01',
  'Proxy > JP - IIJ Dedicated Line',
  'DIRECT',
  'REJECT',
  'Proxy > US - GIA BGP Multi-Hop',
  'Proxy > SG - AWS CloudFront',
  'Proxy > TW - HiNet Static IP',
]
const networks = ['tcp', 'udp']
const inbounds = ['mixed', 'tun0', 'socks5-in', 'http-in']
const rules = ['GEOSITE,cn', 'GEOIP,cn', 'MATCH', 'IP-CIDR,10.0.0.0/8', 'DOMAIN-SUFFIX,google.com']
const domains = [
  'play.google.com',
  'www.youtube-nocookie.com',
  'github-production-release-asset-2e65be.s3.amazonaws.com',
  'cdnjs.cloudflare.com',
  'swsdist-microsoft-com.akamaized.net',
  'a1.mzstatic.com',
  's3-us-west-2.amazonaws.com',
  'cdn.jsdelivr.net',
  'registry.npmjs.org',
  'chat.openai.cdn.cloudflare.net',
  'api.steampowered.com',
  'images-1.wegame.qq.com',
]

function makeOverview(range: string): OverviewResponse {
  const stepMap: Record<string, [number, number]> = {
    '15m': [60, 15],
    '1h': [240, 15],
    '6h': [240, 90],
    '24h': [288, 300],
    '7d': [336, 1800],
  }
  const [count, stepSec] = stepMap[range] || [240, 15]
  return {
    generatedAt: new Date(baseTime).toISOString(),
    sourceState: 'online',
    window: {
      from: new Date(baseTime - count * stepSec * 1000).toISOString(),
      to: new Date(baseTime).toISOString(),
      stepSeconds: stepSec,
    },
    current: {
      observedAt: new Date(baseTime).toISOString(),
      version: '1.13.0-observability',
      uptimeSeconds: 86523.25,
      memoryBytes: 73400320,
      goroutines: 86,
      activeConnections: 12,
      recentConnections: 284,
      connectionsTotal: 18420,
      uploadBytesTotal: 835321120,
      downloadBytesTotal: 5905580032,
      uploadBytesPerSecond: 24576,
      downloadBytesPerSecond: 1048576,
    },
    rangeTotals: {
      uploadBytes: 94371840,
      downloadBytes: 1939865600,
      connections: 721,
    },
    series: generateSeries(count, stepSec),
    topOutbounds: [
      { value: 'Proxy > HK - CMI Premium Node 01', uploadBytesPerSecond: 16384, downloadBytesPerSecond: 786432, activeConnections: 8 },
      { value: 'DIRECT', uploadBytesPerSecond: 8192, downloadBytesPerSecond: 262144, activeConnections: 4 },
      { value: 'Proxy > US - GIA BGP Multi-Hop', uploadBytesPerSecond: 4096, downloadBytesPerSecond: 131072, activeConnections: 3 },
    ],
    urlTests: [
      { outbound: 'Proxy > HK - CMI Premium Node 01', delayMs: 48, measuredAt: new Date(baseTime - 10000).toISOString(), ageSeconds: 10 },
      { outbound: 'Proxy > JP - IIJ Dedicated Line', delayMs: 82, measuredAt: new Date(baseTime - 30000).toISOString(), ageSeconds: 30 },
      { outbound: 'Proxy > US - GIA BGP Multi-Hop', delayMs: 143, measuredAt: new Date(baseTime - 60000).toISOString(), ageSeconds: 60 },
      { outbound: 'Proxy > SG - AWS CloudFront', delayMs: 256, measuredAt: new Date(baseTime - 120000).toISOString(), ageSeconds: 120 },
    ],
    apiHealth: {
      recentConnectionsCapacity: 2000,
      recentConnectionsUtilization: 0.142,
      sseSubscribers: 1,
      sseEventsTotal: 2842,
      sseEventsPerSecond: 1.8,
      errorRate: 0.004,
      endpoints: [
        { endpoint: 'metrics', status: 200, requestsTotal: 43200, requestsPerSecond: 0.5, averageDurationMs: 1.8, responseBytesPerSecond: 8192 },
        { endpoint: 'connections/active', status: 200, requestsTotal: 14400, requestsPerSecond: 0.17, averageDurationMs: 2.6, responseBytesPerSecond: 4096 },
        { endpoint: 'events', status: 200, requestsTotal: 8, requestsPerSecond: 0, averageDurationMs: null, responseBytesPerSecond: 0 },
      ],
    },
  }
}

function generateConnections(count: number, state: 'active' | 'closed'): Connection[] {
  const out: Connection[] = []
  const now = baseTime
  for (let i = 0; i < count; i++) {
    const outbound = outboundNames[i % outboundNames.length]
    const network = networks[i % networks.length]
    const inbound = inbounds[i % inbounds.length]
    const domain = domains[i % domains.length]
    const rule = rules[i % rules.length]
    const start = now - Math.floor(Math.random() * 600000)
    const upload = Math.floor(Math.random() * 5000000)
    const download = Math.floor(Math.random() * 50000000)
    const closedAt = state === 'closed' ? new Date(start + 30_000 + Math.floor(Math.random() * 300_000)).toISOString() : null
    out.push({
      id: `conn-${i}`,
      state,
      startedAt: new Date(start).toISOString(),
      closedAt,
      durationSeconds: closedAt ? (new Date(closedAt).getTime() - start) / 1000 : (now - start) / 1000,
      rule,
      outbound,
      outboundType: outbound === 'DIRECT' ? 'direct' : (outbound === 'REJECT' ? 'block' : 'selector'),
      chain: outbound.split(' > '),
      network,
      inbound,
      domain,
      destinationPort: [443, 80, 8080, 53][i % 4],
      destinationIP: i % 3 === 0 ? undefined : `1.2.3.${i % 254}`,
      sourceIP: `192.168.1.${(i % 20) + 2}`,
      sourcePort: 50000 + i,
      process: i % 4 === 0 ? undefined : ['chrome.exe', 'firefox.exe', 'curl.exe', 'node.exe', 'WeChatAppEx.exe'][i % 5],
      user: i % 5 === 0 ? undefined : 'user',
      upload,
      download,
    })
  }
  return out
}

const activeConnections = generateConnections(12, 'active')
const recentConnections = generateConnections(284, 'closed')

function generateRankings(dim: string, limit: number): RankingEntry[] {
  const values: string[] = (() => {
    switch (dim) {
      case 'network': return networks
      case 'inbound': return inbounds
      case 'outbound': return outboundNames
      case 'rule': return rules
      case 'domain': return domains
      case 'destination_ip': return ['1.2.3.4', '8.8.8.8', '1.1.1.1', '104.16.0.1']
      case 'source': return ['192.168.1.2', '192.168.1.3', '192.168.1.10']
      case 'process': return ['chrome.exe', 'firefox.exe', 'WeChatAppEx.exe', 'curl.exe', 'Telegram.exe']
      case 'user': return ['user']
      default: return outboundNames
    }
  })()
  const out: RankingEntry[] = []
  let remaining = 100
  for (let i = 0; i < Math.min(values.length, limit); i++) {
    const isLast = i === Math.min(values.length, limit) - 1
    const pct = isLast ? remaining : Math.max(1, remaining * (0.5 + Math.random() * 0.2))
    const actualPct = isLast ? remaining : Math.min(remaining - (values.length - i - 1), pct)
    remaining -= actualPct
    out.push({
      value: values[i],
      percentage: actualPct,
      downloadBytes: Math.floor(Math.random() * 500000000 * (1 - i / values.length)),
      uploadBytes: Math.floor(Math.random() * 50000000 * (1 - i / values.length)),
      connections: Math.floor(50 + Math.random() * 500 * (1 - i / values.length)),
      activeConnections: Math.floor(1 + Math.random() * 20 * (1 - i / values.length)),
    })
  }
  return out
}

export function createHandlers(authEnabled = false) {
  let authenticated = false

  function checkAuth(request: Request) {
    if (!authEnabled) return true
    if (authenticated) return true
    const cookie = request.headers.get('Cookie') || ''
    if (cookie.includes('sbox_observability_session') || cookie.includes('console_token')) {
      authenticated = true
      return true
    }
    return false
  }

  return [
    http.get('/healthz', () => HttpResponse.json({ status: 'ok' })),
    http.get('/readyz', () => HttpResponse.json({ status: 'ok' })),

    http.get('/api/v1/session', () => {
      const resp: SessionResponse = {
        authEnabled,
        authenticated,
      }
      return HttpResponse.json(resp)
    }),

    http.post('/api/v1/session', async ({ request }) => {
      const body = await request.json() as { token: string }
      if (body.token === 'password' || body.token === 'token') {
        authenticated = true
        const resp: SessionResponse = { authEnabled, authenticated: true }
        return HttpResponse.json(resp, {
          headers: { 'Set-Cookie': 'sbox_observability_session=mock-session; Path=/; HttpOnly; SameSite=Strict' },
        })
      }
      return HttpResponse.json({ error: { code: 'INVALID_TOKEN', message: 'Invalid token', requestId: 'mock', retryable: false } }, { status: 401 })
    }),

    http.delete('/api/v1/session', () => {
      authenticated = false
      return new HttpResponse(null, { status: 204, headers: { 'Set-Cookie': 'sbox_observability_session=; Path=/; Max-Age=0' } })
    }),

    http.get('/api/v1/meta', ({ request }) => {
      if (!checkAuth(request)) {
        return HttpResponse.json({ error: { code: 'UNAUTHORIZED', message: 'Authentication required', requestId: 'mock', retryable: false } }, { status: 401 })
      }
      return HttpResponse.json(metaOnline)
    }),

    http.get('/api/v1/overview', ({ request }) => {
      if (!checkAuth(request)) {
        return HttpResponse.json({ error: { code: 'UNAUTHORIZED', message: 'Authentication required', requestId: 'mock', retryable: false } }, { status: 401 })
      }
      const url = new URL(request.url)
      const range = url.searchParams.get('range') || '1h'
      return HttpResponse.json(makeOverview(range))
    }),

    http.get('/api/v1/rankings', ({ request }) => {
      if (!checkAuth(request)) {
        return HttpResponse.json({ error: { code: 'UNAUTHORIZED', message: 'Authentication required', requestId: 'mock', retryable: false } }, { status: 401 })
      }
      const url = new URL(request.url)
      const dim = url.searchParams.get('dimension') || 'outbound'
      const sort = url.searchParams.get('sort') || 'traffic'
      const limit = parseInt(url.searchParams.get('limit') || '10', 10)
      const sensitive = ['domain', 'destination_ip', 'source', 'process', 'user']
      if (sensitive.includes(dim) && !metaOnline.capabilities?.exposeSensitive) {
        return HttpResponse.json({ error: { code: 'SENSITIVE_DIMENSION_DISABLED', message: 'Sensitive dimension is disabled', requestId: 'mock', retryable: false } }, { status: 403 })
      }
      return HttpResponse.json({
        generatedAt: new Date(baseTime).toISOString(),
        window: {
          from: new Date(baseTime - 3600_000).toISOString(),
          to: new Date(baseTime).toISOString(),
          stepSeconds: 15,
        },
        dimension: dim,
        sort,
        total: generateRankings(dim, 100).length,
        data: generateRankings(dim, limit),
      })
    }),

    http.get('/api/v1/dimensions/series', ({ request }) => {
      if (!checkAuth(request)) {
        return HttpResponse.json({ error: { code: 'UNAUTHORIZED', message: 'Authentication required', requestId: 'mock', retryable: false } }, { status: 401 })
      }
      const url = new URL(request.url)
      const dimension = url.searchParams.get('dimension') || 'outbound'
      const value = url.searchParams.get('value') || 'DIRECT'
      const range = url.searchParams.get('range') || '1h'
      const overview = makeOverview(range)
      return HttpResponse.json({
        generatedAt: new Date(baseTime).toISOString(),
        window: overview.window,
        dimension,
        value,
        series: overview.series.map((item) => ({
          timestamp: item.timestamp,
          uploadBytesPerSecond: item.uploadBytesPerSecond == null ? null : item.uploadBytesPerSecond * 0.35,
          downloadBytesPerSecond: item.downloadBytesPerSecond == null ? null : item.downloadBytesPerSecond * 0.35,
          connectionsPerSecond: item.activeConnections == null ? null : item.activeConnections / 10,
          activeConnections: item.activeConnections == null ? null : Math.max(0, Math.round(item.activeConnections * 0.35)),
          delayMs: 80 + Math.round(Math.random() * 60),
        })),
      })
    }),

    http.get('/api/v1/connections/active', ({ request }) => {
      if (!checkAuth(request)) {
        return HttpResponse.json({ error: { code: 'UNAUTHORIZED', message: 'Authentication required', requestId: 'mock', retryable: false } }, { status: 401 })
      }
      const url = new URL(request.url)
      const q = url.searchParams.get('q')?.toLowerCase()
      let conns = activeConnections
      if (q) {
        conns = conns.filter(c =>
          c.outbound.toLowerCase().includes(q) ||
          c.domain?.toLowerCase().includes(q) ||
          c.rule?.toLowerCase().includes(q) ||
          c.network.includes(q),
        )
      }
      const limit = parseInt(url.searchParams.get('limit') || '50', 10)
      const offset = parseInt(url.searchParams.get('offset') || '0', 10)
      return HttpResponse.json({
        generatedAt: new Date(baseTime).toISOString(),
        total: conns.length,
        limit,
        offset,
        data: conns.slice(offset, offset + limit),
      })
    }),

    http.get('/api/v1/connections/recent', ({ request }) => {
      if (!checkAuth(request)) {
        return HttpResponse.json({ error: { code: 'UNAUTHORIZED', message: 'Authentication required', requestId: 'mock', retryable: false } }, { status: 401 })
      }
      const url = new URL(request.url)
      const q = url.searchParams.get('q')?.toLowerCase()
      let conns = recentConnections
      if (q) {
        conns = conns.filter(c =>
          c.outbound.toLowerCase().includes(q) ||
          c.domain?.toLowerCase().includes(q) ||
          c.rule?.toLowerCase().includes(q),
        )
      }
      const limit = parseInt(url.searchParams.get('limit') || '50', 10)
      const offset = parseInt(url.searchParams.get('offset') || '0', 10)
      return HttpResponse.json({
        generatedAt: new Date(baseTime).toISOString(),
        total: conns.length,
        limit,
        offset,
        data: conns.slice(offset, offset + limit),
      })
    }),

    http.get('/api/v1/events', () => {
      const stream = new ReadableStream({
        start(controller) {
          const encoder = new TextEncoder()
          const events = [
            `id: 1\nevent: hello\ndata: ${JSON.stringify({ sequence: 1, generatedAt: new Date(baseTime).toISOString(), type: 'hello', data: {} })}\n\n`,
            `id: 2\nevent: source.state\ndata: ${JSON.stringify({ sequence: 2, generatedAt: new Date(baseTime + 100).toISOString(), type: 'source.state', data: { state: 'online', lastSuccessAt: new Date(baseTime).toISOString(), errorCode: null } })}\n\n`,
          ]
          let idx = 0
          const send = () => {
            if (idx < events.length) {
              controller.enqueue(encoder.encode(events[idx]))
              idx++
              setTimeout(send, 100)
            }
          }
          send()
        },
      })
      return new HttpResponse(stream, {
        headers: {
          'Content-Type': 'text/event-stream',
          'Cache-Control': 'no-cache',
          'Connection': 'keep-alive',
        },
      })
    }),
  ]
}

export const handlers = createHandlers(false)
