import { useState, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { useActiveConnections, useRecentConnections } from '@/api/hooks'
import type { Connection, TimeRange } from '@/api/types'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { PageHeader } from '@/components/layout/page-header'
import { EmptyState, ErrorState } from '@/components/data-state/states'
import { formatBytes, formatLocalDateTime, formatCount, formatDuration } from '@/lib/format'
import { Search, ChevronDown, X } from 'lucide-react'
import { useSearchParams } from 'react-router-dom'

const PAGE_SIZE = 50

function destinationHost(connection: Connection): string {
  return connection.domain || connection.destinationIP || '—'
}

function destinationLabel(connection: Connection): string {
  const host = destinationHost(connection)
  return host === '—' ? `:${connection.destinationPort}` : `${host}:${connection.destinationPort}`
}

function useDebounce<T>(value: T, delay: number): T {
  const [debouncedValue, setDebouncedValue] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedValue(value), delay)
    return () => clearTimeout(timer)
  }, [value, delay])
  return debouncedValue
}

function useIsMobile() {
  const [isMobile, setIsMobile] = useState(() => window.matchMedia('(max-width: 719px)').matches)
  useEffect(() => {
    const query = window.matchMedia('(max-width: 719px)')
    const handler = (event: MediaQueryListEvent) => setIsMobile(event.matches)
    query.addEventListener('change', handler)
    return () => query.removeEventListener('change', handler)
  }, [])
  return isMobile
}

export function ConnectionsPage() {
  const { t, i18n } = useTranslation()
  const locale = i18n.language
  const isMobile = useIsMobile()
  const [urlParams, setUrlParams] = useSearchParams()
  const [tab, setTab] = useState<'active' | 'recent'>(() => urlParams.get('tab') === 'recent' ? 'recent' : 'active')
  const [search, setSearch] = useState(() => urlParams.get('q') ?? '')
  const [network, setNetwork] = useState(() => urlParams.get('network') ?? '')
  const [outbound, setOutbound] = useState(() => urlParams.get('outbound') ?? '')
  const [page, setPage] = useState(() => Math.max(1, Number.parseInt(urlParams.get('page') ?? '1', 10) || 1))
  const [selected, setSelected] = useState<Connection | null>(null)
  const debouncedSearch = useDebounce(search, 300)

  useEffect(() => { setPage(1) }, [debouncedSearch, tab, network, outbound])

  useEffect(() => {
    const next = new URLSearchParams()
    next.set('tab', tab)
    if (debouncedSearch) next.set('q', debouncedSearch)
    if (network) next.set('network', network)
    if (outbound) next.set('outbound', outbound)
    next.set('page', String(page))
    if (next.toString() !== urlParams.toString()) {
      setUrlParams(next, { replace: true })
    }
  }, [debouncedSearch, network, outbound, page, setUrlParams, tab, urlParams])

  const filters = { q: debouncedSearch || undefined, network: network || undefined, outbound: outbound || undefined, page, limit: PAGE_SIZE }
  const activeQuery = useActiveConnections(filters)
  const recentQuery = useRecentConnections({ range: '1h' as TimeRange, ...filters })

  const { data, isLoading, error, refetch, isFetching } = tab === 'active' ? activeQuery : recentQuery

  const connections = data?.data ?? []
  const total = data?.total ?? 0
  const totalPages = Math.ceil(total / PAGE_SIZE)
  const hasMore = page < totalPages

  if (isMobile && selected) {
    return (
      <div className="min-w-0">
        <PageHeader title={t('connections.details')} back={{ label: t('connections.title'), onClick: () => setSelected(null) }} />
        <ConnectionDetails connection={selected} />
      </div>
    )
  }

  return (
    <div className="min-w-0">
      <PageHeader title={t('connections.title')} />

      <Card className="max-lg:overflow-visible max-lg:border-0 max-lg:bg-transparent max-lg:shadow-none">
        <CardHeader className="pb-2">
          <Tabs value={tab} onValueChange={(v) => setTab(v as 'active' | 'recent')} className="w-full">
            <div className="flex flex-col gap-2">
              <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2">
                <TabsList>
                  <TabsTrigger value="active">{t('connections.activeTab')}</TabsTrigger>
                  <TabsTrigger value="recent">{t('connections.recentTab')}</TabsTrigger>
                </TabsList>
                <div className="relative">
                  <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--color-text-faint)]" />
                  <Input
                    className="w-full pl-9 sm:w-64"
                    placeholder={t('connections.searchPlaceholder')}
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                  />
                  {search && (
                    <button
                      className="absolute right-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-faint)] hover:text-[var(--color-text)]"
                      onClick={() => setSearch('')}
                      aria-label={t('connections.clearSearch')}
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                  )}
                </div>
              </div>
              <div className="grid grid-cols-2 gap-2 sm:flex sm:justify-end">
                <Input className="sm:w-40" aria-label={t('connections.network')} placeholder={t('connections.network')} value={network} onChange={(event) => setNetwork(event.target.value)} />
                <Input className="sm:w-48" aria-label={t('connections.outbound')} placeholder={t('connections.outbound')} value={outbound} onChange={(event) => setOutbound(event.target.value)} />
              </div>
            </div>
          </Tabs>
        </CardHeader>
        <CardContent>
          {isLoading && (
            <div className="space-y-2">
              {Array.from({ length: 8 }).map((_, i) => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          )}

          {error && !data && <ErrorState error={error as Error} onRetry={refetch} />}

          {data && connections.length === 0 && !isLoading && (
            <EmptyState title={t('connections.noResults')} />
          )}

          {data && connections.length > 0 && (
            <>
              {isFetching && !isLoading && (
                <div className="mb-2 h-0.5 overflow-hidden rounded bg-[var(--color-inset)]">
                  <div className="h-full w-1/3 animate-pulse bg-[var(--color-text-faint)]" />
                </div>
              )}

              {isMobile ? (
                <ConnectionCards connections={connections} onSelect={setSelected} />
              ) : (
                <VirtualConnectionTable connections={connections} onSelect={setSelected} />
              )}

              {total > PAGE_SIZE && (
                <div className="mt-4 flex items-center justify-center gap-2">
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => setPage((p) => p - 1)}
                    disabled={page <= 1}
                  >
                    {t('common.back')}
                  </Button>
                  <span className="text-xs text-[var(--color-text-muted)]">
                    {page} / {totalPages} · {formatCount(total, locale)} {t('common.total')}
                  </span>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => setPage((p) => p + 1)}
                    disabled={!hasMore}
                  >
                    {t('connections.loadMore')}
                    <ChevronDown className="ml-1 h-3 w-3" />
                  </Button>
                </div>
              )}

              {total > 0 && (
                <div className="mt-4 text-xs text-center text-[var(--color-text-faint)]">
                  {formatCount(total, locale)} {tab === 'active' ? t('connections.activeTab') : t('connections.recentTab')}
                </div>
              )}
            </>
          )}
        </CardContent>
      </Card>

      <Sheet open={!!selected} onOpenChange={(o) => !o && setSelected(null)}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>{t('connections.details')}</SheetTitle>
          </SheetHeader>
          {selected && <ConnectionDetails connection={selected} />}
        </SheetContent>
      </Sheet>
    </div>
  )
}

function ConnectionCards({ connections, onSelect }: { connections: Connection[]; onSelect: (c: Connection) => void }) {
  return (
    <div className="space-y-2 min-w-0">
      {connections.map((conn) => (
        <button
          key={conn.id}
          onClick={() => onSelect(conn)}
          className="surface-card w-full p-3 text-left transition-[border-color,background] hover:border-[var(--color-border-strong)] active:bg-[var(--color-hover)] focus:outline-none min-w-0 overflow-hidden"
        >
          <div className="flex items-start justify-between gap-2 mb-1 min-w-0">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-1.5 min-w-0">
                <Badge variant={conn.network === 'tcp' ? 'primary' : 'download'} className="flex-shrink-0 text-[10px]">
                  {conn.network}
                </Badge>
                <span className="font-mono text-xs truncate min-w-0" title={destinationLabel(conn)}>
                  {destinationLabel(conn)}
                </span>
              </div>
              <div className="text-[11px] text-[var(--color-text-faint)] mt-0.5 flex items-center gap-1.5 min-w-0">
                <span className="truncate min-w-0" title={conn.outbound}>{conn.outbound}</span>
                <span className="flex-shrink-0 inline-flex items-center">
                  <span className={conn.state === 'active' ? 'h-1.5 w-1.5 rounded-full bg-[var(--color-healthy)] animate-pulse' : 'h-1.5 w-1.5 rounded-full bg-[var(--color-text-faint)]'} />
                </span>
              </div>
            </div>
            <div className="text-right flex-shrink-0">
              <div className="flex items-center justify-end gap-1 text-xs text-[var(--color-text-muted)]">
                <span aria-hidden="true">↓</span>
                <span className="whitespace-nowrap tabular-nums">{formatBytes(conn.download, 1)}</span>
              </div>
              <div className="flex items-center justify-end gap-1 text-xs text-[var(--color-text-muted)]">
                <span aria-hidden="true">↑</span>
                <span className="whitespace-nowrap tabular-nums">{formatBytes(conn.upload, 1)}</span>
              </div>
            </div>
          </div>
          <div className="flex min-w-0 items-center gap-1 overflow-hidden text-[11px] text-[var(--color-text-muted)]">
            <span className="min-w-0 truncate font-mono">{conn.rule || conn.inbound || '—'}</span>
          </div>
        </button>
      ))}
    </div>
  )
}

function VirtualConnectionTable({ connections, onSelect }: { connections: Connection[]; onSelect: (c: Connection) => void }) {
  const { t } = useTranslation()
  const parentRef = useRef<HTMLDivElement>(null)
  const [scrollTop, setScrollTop] = useState(0)
  const rowHeight = 40
  const headerHeight = 36
  const viewportHeight = Math.min(600, Math.max(300, connections.length * rowHeight + headerHeight))

  useEffect(() => {
    const el = parentRef.current
    if (!el) return
    const handler = () => setScrollTop(el.scrollTop)
    el.addEventListener('scroll', handler, { passive: true })
    return () => el.removeEventListener('scroll', handler)
  }, [])

  const startIndex = Math.max(0, Math.floor(scrollTop / rowHeight) - 5)
  const endIndex = Math.min(connections.length, Math.ceil((scrollTop + viewportHeight) / rowHeight) + 5)
  const visible = connections.slice(startIndex, endIndex)
  const totalHeight = connections.length * rowHeight

  return (
    <div className="border border-[var(--color-border)] rounded-lg overflow-hidden">
      <div
        ref={parentRef}
        className="overflow-auto relative"
        style={{ height: viewportHeight }}
        role="table"
      >
        <div className="sticky top-0 z-10 grid bg-[var(--color-surface-2)] text-[11px] font-semibold uppercase tracking-wider text-[var(--color-text-faint)] border-b border-[var(--color-border)]"
          style={{
            gridTemplateColumns: 'minmax(180px,2fr) 100px 70px 100px 1fr 120px 90px',
            height: headerHeight,
          }}
        >
          <div className="px-3 flex items-center">{t('connections.destination')}</div>
          <div className="px-2 flex items-center">{t('connections.network')}</div>
          <div className="px-2 flex items-center">{t('connections.inbound')}</div>
          <div className="px-2 flex items-center">{t('connections.outbound')}</div>
          <div className="px-2 flex items-center truncate">{t('connections.rule')}</div>
          <div className="px-2 flex items-center justify-end">{t('common.download')}</div>
          <div className="px-2 flex items-center justify-end">{t('common.upload')}</div>
        </div>
        <div style={{ height: totalHeight, position: 'relative' }}>
          {visible.map((conn, i) => {
            const idx = startIndex + i
            const dest = destinationLabel(conn)
            return (
              <button
                key={conn.id}
                role="row"
                onClick={() => onSelect(conn)}
                className="absolute left-0 right-0 grid items-center border-b border-[var(--color-border)] hover:bg-[var(--color-surface-2)] transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)] focus-visible:ring-inset text-sm text-left px-3"
                style={{
                  top: idx * rowHeight,
                  height: rowHeight,
                  gridTemplateColumns: 'minmax(180px,2fr) 100px 70px 100px 1fr 120px 90px',
                }}
              >
                <div className="px-0 flex items-center gap-2 min-w-0">
                  <span className="h-1.5 w-1.5 rounded-full bg-[var(--color-healthy)] flex-shrink-0" />
                  <span className="font-mono text-xs truncate" title={dest}>{dest}</span>
                </div>
                <div className="px-0">
                  <Badge variant="outline" className="text-[10px]">{conn.network}</Badge>
                </div>
                <div className="px-0 font-mono text-xs text-[var(--color-text-muted)] truncate" title={conn.inbound}>{conn.inbound}</div>
                <div className="px-0 font-mono text-xs truncate" title={conn.outbound}>{conn.outbound}</div>
                <div className="px-0 font-mono text-xs text-[var(--color-text-muted)] truncate" title={conn.rule}>{conn.rule || '—'}</div>
                <div className="px-0 text-right font-mono text-xs tabular-nums">
                  {formatBytes(conn.download, 1)}
                </div>
                <div className="px-0 text-right font-mono text-xs tabular-nums">
                  {formatBytes(conn.upload, 1)}
                </div>
              </button>
            )
          })}
        </div>
      </div>
    </div>
  )
}

function ConnectionDetails({ connection: conn }: { connection: Connection }) {
  const { t, i18n } = useTranslation()
  const locale = i18n.language

  const rows: { label: string; value: React.ReactNode }[] = [
    { label: t('connections.connectionId'), value: <span className="font-mono text-xs break-all">{conn.id}</span> },
    { label: t('connections.network'), value: <Badge variant="outline">{conn.network}</Badge> },
    { label: t('connections.inbound'), value: <span className="font-mono">{conn.inbound}</span> },
    { label: t('connections.outbound'), value: <span className="font-mono">{conn.outbound}</span> },
    { label: t('connections.outboundType'), value: <span className="font-mono">{conn.outboundType}</span> },
  ]

  const source = conn.sourceHostname || conn.sourceIP
  if (source) rows.push({ label: t('connections.source'), value: <span className="font-mono text-xs">{source}{conn.sourcePort !== undefined ? `:${conn.sourcePort}` : ''}</span> })
  rows.push({ label: t('connections.destination'), value: <span className="font-mono text-xs">{destinationLabel(conn)}</span> })
  if (conn.rule) rows.push({ label: t('connections.rule'), value: <span className="font-mono text-xs break-all">{conn.rule}</span> })
  if (conn.chain.length > 0) rows.push({ label: t('connections.chain'), value: <span className="font-mono text-xs break-all">{conn.chain.join(' › ')}</span> })
  if (conn.process) rows.push({ label: t('connections.process'), value: <span className="font-mono text-xs break-all">{conn.process}</span> })
  if (conn.user) rows.push({ label: t('connections.user'), value: <span className="font-mono text-xs">{conn.user}</span> })

  return (
    <div className="space-y-4 pt-2">
      <div className="grid grid-cols-2 gap-3">
        <div className="rounded-lg bg-[var(--color-surface-2)] p-3">
          <div className="text-[11px] font-semibold uppercase tracking-wider text-[var(--color-text-faint)] mb-1">
            {t('common.download')}
          </div>
          <div className="text-lg font-medium tabular-nums">
             {formatBytes(conn.download)}
          </div>
        </div>
        <div className="rounded-lg bg-[var(--color-surface-2)] p-3">
          <div className="text-[11px] font-semibold uppercase tracking-wider text-[var(--color-text-faint)] mb-1">
            {t('common.upload')}
          </div>
          <div className="text-lg font-medium tabular-nums">
             {formatBytes(conn.upload)}
          </div>
        </div>
      </div>

      <div className="space-y-2">
        {rows.map((r) => (
          <div key={r.label} className="flex items-start justify-between gap-4 py-1.5 border-b border-[var(--color-border)] last:border-0">
            <span className="text-xs text-[var(--color-text-faint)] flex-shrink-0 w-28">{r.label}</span>
            <span className="text-right min-w-0">{r.value}</span>
          </div>
        ))}
        <div className="flex items-start justify-between gap-4 py-1.5">
          <span className="text-xs text-[var(--color-text-faint)] flex-shrink-0 w-28">{t('connections.started')}</span>
          <span className="font-mono text-xs text-right">{formatLocalDateTime(conn.startedAt, locale)}</span>
        </div>
        {conn.closedAt && (
          <div className="flex items-start justify-between gap-4 py-1.5">
            <span className="text-xs text-[var(--color-text-faint)] flex-shrink-0 w-28">{t('connections.closed')}</span>
            <span className="font-mono text-xs text-right">{formatLocalDateTime(conn.closedAt, locale)}</span>
          </div>
        )}
        <div className="flex items-start justify-between gap-4 py-1.5">
          <span className="text-xs text-[var(--color-text-faint)] flex-shrink-0 w-28">{t('connections.duration')}</span>
          <span className="font-mono text-xs text-right">{formatDuration(conn.durationSeconds)}</span>
        </div>
      </div>
    </div>
  )
}
