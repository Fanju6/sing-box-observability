/**
 * Dashboard composition adapted in part from sing-box-dashboard.
 * Copyright (C) 2022 nekohasekai <contact-sagernet@sekai.icu>.
 * Modifications Copyright (C) 2026 Fanju and contributors.
 * See NOTICE and THIRD_PARTY_LICENSES.txt.
 */
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  AlertTriangle,
  ArrowDown,
  ArrowUp,
  Cable,
  ChevronRight,
  CircleGauge,
  Gauge,
  SlidersHorizontal,
  Timer,
  Wifi,
} from 'lucide-react'
import { useMeta, useOverview } from '@/api/hooks'
import { Card } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { TrafficSparkline } from '@/components/charts/traffic-chart'
import { CardSkeleton, ErrorState } from '@/components/data-state/states'
import { PageHeader } from '@/components/layout/page-header'
import { formatByteRate, formatBytes, formatCount, formatDelay, formatDuration, formatPercent } from '@/lib/format'
import type { SourceState } from '@/api/types'
import { useSSEState } from '@/app/use-sse-state'

const channelKeys = ['capabilities', 'metrics', 'connections', 'events'] as const
const overviewItems = [
  { id: 'upload', label: 'common.upload', icon: ArrowUp },
  { id: 'download', label: 'common.download', icon: ArrowDown },
  { id: 'runtime', label: 'overview.runtime', icon: Gauge },
  { id: 'connections', label: 'common.connections', icon: Cable },
  { id: 'topOutbounds', label: 'overview.topOutbounds', icon: Wifi },
  { id: 'urlTests', label: 'overview.urlTests', icon: Timer },
  { id: 'diagnostics', label: 'overview.diagnostics', icon: CircleGauge },
] as const

type OverviewItem = (typeof overviewItems)[number]['id']
const defaultItems = overviewItems.map((item) => item.id)
const storageKey = 'observability-overview-items-v2'

function stateVariant(state: SourceState) {
  if (state === 'online') return 'healthy' as const
  if (state === 'stale' || state === 'connecting') return 'warning' as const
  return 'danger' as const
}

function loadItems(): OverviewItem[] {
  try {
    if (typeof window === 'undefined') return defaultItems
    const stored = JSON.parse(window.localStorage.getItem(storageKey) ?? 'null')
    if (!Array.isArray(stored)) return defaultItems
    const known = new Set(defaultItems)
    return stored.filter((item): item is OverviewItem => typeof item === 'string' && known.has(item as OverviewItem))
  } catch {
    return defaultItems
  }
}

export function OverviewPage() {
  const { t, i18n } = useTranslation()
  const locale = i18n.language
  const { data: meta } = useMeta()
  const { data, isLoading, error, refetch } = useOverview('1h')
  const browserStream = useSSEState()
  const [managing, setManaging] = useState(false)
  const [enabledItems, setEnabledItems] = useState<OverviewItem[]>(loadItems)
  const visible = (item: OverviewItem) => enabledItems.includes(item)

  const updateItems = (next: OverviewItem[]) => {
    setEnabledItems(next)
    try {
      window.localStorage.setItem(storageKey, JSON.stringify(next))
    } catch {
      // Keep the in-memory preference when storage is unavailable.
    }
  }

  if (isLoading) {
    return (
      <div>
        <PageHeading onManage={() => setManaging(true)} />
        <div className="grid grid-cols-2 gap-2.5 lg:gap-3.5"><CardSkeleton /><CardSkeleton /><CardSkeleton /><CardSkeleton /></div>
      </div>
    )
  }
  if (error && !data) {
    return <div><PageHeading onManage={() => setManaging(true)} /><ErrorState error={error as Error} onRetry={refetch} /></div>
  }
  if (!data) return <PageHeading onManage={() => setManaging(true)} />

  const current = data.current
  const totals = data.rangeTotals
  const health = data.apiHealth
  const channels = meta?.collector.channels

  return (
    <div>
      <PageHeading onManage={() => setManaging(true)} />

      <div className="space-y-4">
      {data.sourceState !== 'online' && (
        <div className="flex items-start gap-2.5 rounded-xl border border-[var(--color-warning-soft)] bg-[var(--color-warning-soft)] p-3 text-[13px]">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-[var(--color-warning)]" />
          <div>
            <div className="font-medium">{t(`source.${data.sourceState}`)}</div>
            {meta?.source.lastErrorCode && <div className="mt-0.5 font-mono text-[11px] text-[var(--color-text-muted)]">{meta.source.lastErrorCode}</div>}
          </div>
        </div>
      )}

      <div className="grid grid-cols-2 gap-2.5 max-[360px]:grid-cols-1 lg:gap-3.5">
        {visible('upload') && (
          <TrafficMetricCard icon={ArrowUp} title={t('common.upload')} rate={current?.uploadBytesPerSecond} total={totals?.uploadBytes} data={data.series} dataKey="uploadBytesPerSecond" />
        )}
        {visible('download') && (
          <TrafficMetricCard icon={ArrowDown} title={t('common.download')} rate={current?.downloadBytesPerSecond} total={totals?.downloadBytes} data={data.series} dataKey="downloadBytesPerSecond" />
        )}

        {visible('runtime') && (
          <DashboardCard icon={Gauge} title={t('overview.runtime')}>
            <DashboardDataLine label={t('overview.memory')} value={formatBytes(current?.memoryBytes, 1)} />
            <DashboardDataLine label={t('overview.goroutines')} value={formatCount(current?.goroutines, locale)} />
            <DashboardDataLine label={t('overview.uptime')} value={current ? formatDuration(current.uptimeSeconds) : '—'} />
          </DashboardCard>
        )}

        {visible('connections') && (
          <DashboardCard icon={Cable} title={t('common.connections')}>
            <DashboardDataLine label={t('common.active')} value={formatCount(current?.activeConnections, locale)} />
            <DashboardDataLine label={t('common.recent')} value={formatCount(current?.recentConnections, locale)} />
            <DashboardDataLine label={t('overview.rangeConns')} value={formatCount(totals?.connections, locale)} />
          </DashboardCard>
        )}

        {visible('topOutbounds') && (
          <DashboardCard icon={Wifi} title={t('overview.topOutbounds')} wide>
              {data.topOutbounds.length === 0 && <div className="py-5 text-center text-xs text-[var(--color-text-faint)]">{t('common.empty')}</div>}
              {data.topOutbounds.slice(0, 6).map((item, index) => {
                const traffic = (item.downloadBytesPerSecond ?? 0) + (item.uploadBytesPerSecond ?? 0)
                return (
                  <DashboardDataLine
                    key={item.value}
                    label={<span className="flex min-w-0 items-baseline gap-2"><span className="w-4 shrink-0 font-mono text-[10px] text-[var(--color-text-faint)]">{index + 1}</span><span className="truncate font-mono text-xs text-[var(--color-text)]">{item.value}</span></span>}
                    value={formatByteRate(traffic)}
                  />
                )
              })}
          </DashboardCard>
        )}

        {visible('urlTests') && (
          <DashboardCard icon={Timer} title={t('overview.urlTests')} wide>
              {data.urlTests.length === 0 && <div className="py-5 text-center text-xs text-[var(--color-text-faint)]">{t('overview.noUrlTests')}</div>}
              {data.urlTests.slice(0, 6).map((test) => (
                <DashboardDataLine
                  key={test.outbound}
                  label={<span className="block min-w-0 truncate font-mono text-xs text-[var(--color-text)]" title={test.outbound}>{test.outbound}</span>}
                  value={<Badge variant={test.delayMs < 250 ? 'healthy' : test.delayMs < 800 ? 'warning' : 'danger'}>{formatDelay(test.delayMs, locale)}</Badge>}
                />
              ))}
          </DashboardCard>
        )}

        {visible('diagnostics') && (channels || health) && (
          <details className="surface-card group col-span-full">
            <summary className="flex cursor-pointer list-none items-center gap-2 p-3.5 text-[13px] font-semibold lg:p-5">
              <CircleGauge className="h-4 w-4 text-[var(--color-text-faint)]" />
              {t('overview.diagnostics')}
              <ChevronRight className="ml-auto h-4 w-4 text-[var(--color-text-faint)] transition-transform group-open:rotate-90" />
            </summary>
            <div className="grid gap-5 border-t border-[var(--color-border)] px-3.5 pb-3.5 pt-4 lg:grid-cols-2 lg:px-5 lg:pb-5">
              {channels && (
                <div>
                  <div className="section-label mb-2">{t('overview.collectorChannels')}</div>
                  <div className="divide-y divide-[var(--color-border)]">
                    {channelKeys.map((key) => (
                      <div key={key} className="flex items-center justify-between py-2 text-xs">
                        <span>{t(`overview.channel.${key}`)}</span>
                        <Badge variant={stateVariant(channels[key].state)}>{t(`common.${channels[key].state}`)}</Badge>
                      </div>
                    ))}
                    <div className="flex items-center justify-between py-2 text-xs">
                      <span>{t('overview.channel.browser')}</span>
                      <div className="flex items-center gap-2">
                        {browserStream.lastSequence != null && <span className="font-mono text-[10px] text-[var(--color-text-faint)]">#{browserStream.lastSequence}</span>}
                        <Badge variant={stateVariant(browserStream.state)}>{t(`common.${browserStream.state}`)}</Badge>
                      </div>
                    </div>
                  </div>
                </div>
              )}
              {health && (
                <div>
                  <div className="section-label mb-2">{t('overview.apiHealth')}</div>
                  <div className="divide-y divide-[var(--color-border)]">
                    <DiagnosticRow label={t('overview.recentBuffer')} value={health.recentConnectionsUtilization == null ? '—' : formatPercent(health.recentConnectionsUtilization * 100)} />
                    <DiagnosticRow label={t('overview.apiErrorRate')} value={health.errorRate == null ? '—' : formatPercent(health.errorRate * 100)} />
                    <DiagnosticRow label={t('overview.sseSubscribers')} value={formatCount(health.sseSubscribers, locale)} />
                    <DiagnosticRow label={t('overview.sseEventRate')} value={health.sseEventsPerSecond == null ? '—' : `${health.sseEventsPerSecond.toFixed(2)}/s`} />
                  </div>
                </div>
              )}
            </div>
          </details>
        )}
      </div>
      </div>

      <DashboardItemsDialog open={managing} enabled={enabledItems} onChange={updateItems} onClose={() => setManaging(false)} />
    </div>
  )
}

function PageHeading({ onManage }: { onManage: () => void }) {
  const { t } = useTranslation()
  return (
    <PageHeader
      title={t('overview.title')}
      actions={
      <Button type="button" variant="ghost" size="icon" title={t('overview.dashboardItems')} aria-label={t('overview.dashboardItems')} onClick={onManage}>
        <SlidersHorizontal className="h-[18px] w-[18px]" />
      </Button>
      }
    />
  )
}

function DashboardCard({ icon: Icon, title, wide = false, children }: {
  icon: React.ElementType
  title: string
  wide?: boolean
  children: React.ReactNode
}) {
  return (
    <Card data-dashboard-wide={wide || undefined} className={wide ? 'col-span-full p-3.5 lg:p-5' : 'p-3.5 lg:p-5'}>
      <div className="mb-2.5 flex items-center gap-2 text-[13px] font-semibold">
        <Icon className="h-4 w-4 shrink-0 text-[var(--color-text-faint)]" strokeWidth={1.8} />
        <span>{title}</span>
      </div>
      {children}
    </Card>
  )
}

function TrafficMetricCard({ icon: Icon, title, rate, total, data, dataKey }: {
  icon: React.ElementType
  title: string
  rate: number | null | undefined
  total: number | null | undefined
  data: Array<{ timestamp: string; downloadBytesPerSecond?: number | null; uploadBytesPerSecond?: number | null }>
  dataKey: 'downloadBytesPerSecond' | 'uploadBytesPerSecond'
}) {
  return (
    <DashboardCard icon={Icon} title={title}>
      <div className="truncate text-[22px] font-semibold leading-tight tabular-nums tracking-[-0.01em]">{formatByteRate(rate)}</div>
      <div className="mb-2 mt-px text-[13px] tabular-nums text-[var(--color-text-muted)]">{formatBytes(total)}</div>
      <TrafficSparkline data={data} dataKey={dataKey} height={46} />
    </DashboardCard>
  )
}

function DashboardDataLine({ label, value }: { label: React.ReactNode; value: React.ReactNode }) {
  return (
    <div className="flex min-w-0 items-baseline justify-between gap-3 py-1 text-[13px]">
      <span className="min-w-0 flex-1 text-[var(--color-text-muted)]">{label}</span>
      <span className="shrink-0 text-right font-medium tabular-nums">{value}</span>
    </div>
  )
}

function DashboardItemsDialog({ open, enabled, onChange, onClose }: { open: boolean; enabled: OverviewItem[]; onChange: (items: OverviewItem[]) => void; onClose: () => void }) {
  const { t } = useTranslation()
  const toggle = (item: OverviewItem) => {
    onChange(enabled.includes(item) ? enabled.filter((entry) => entry !== item) : defaultItems.filter((entry) => entry === item || enabled.includes(entry)))
  }
  return (
    <Dialog open={open} onOpenChange={(next) => { if (!next) onClose() }}>
      <DialogContent>
        <DialogHeader className="text-left">
          <DialogTitle className="font-serif text-[19px]">{t('overview.dashboardItems')}</DialogTitle>
          <DialogDescription className="sr-only">{t('overview.dashboardItemsDescription')}</DialogDescription>
        </DialogHeader>
        <div className="divide-y divide-[var(--color-border)]">
          {overviewItems.map((item) => {
            const Icon = item.icon
            const checked = enabled.includes(item.id)
            return (
              <div key={item.id} className="flex items-center gap-2.5 px-1 py-2.5">
                <Icon className="h-[15px] w-[15px] text-[var(--color-text-muted)]" />
                <span className="flex-1 text-[13px] font-medium">{t(item.label)}</span>
                <button type="button" className="dashboard-switch" role="switch" aria-checked={checked} aria-label={t(item.label)} onClick={() => toggle(item.id)} />
              </div>
            )
          })}
        </div>
        <DialogFooter className="flex-row items-center gap-2 space-x-0">
          <Button type="button" variant="ghost" className="mr-auto text-[var(--color-danger)]" onClick={() => onChange(defaultItems)}>{t('common.reset')}</Button>
          <Button type="button" onClick={onClose}>{t('common.done')}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function DiagnosticRow({ label, value }: { label: string; value: string }) {
  return <div className="flex items-center justify-between py-2 text-xs"><span className="text-[var(--color-text-muted)]">{label}</span><span className="data-value">{value}</span></div>
}
