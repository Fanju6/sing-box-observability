import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Activity, ArrowDown, ArrowUp, Database } from 'lucide-react'
import { useDimensionSeries, useOverview, useRankings } from '@/api/hooks'
import type { TimeRange } from '@/api/types'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { TrafficChart } from '@/components/charts/traffic-chart'
import { MultiLineChart } from '@/components/charts/line-chart'
import { ChartSkeleton, ErrorState } from '@/components/data-state/states'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { PageHeader } from '@/components/layout/page-header'
import { formatBytesCompact, formatCount, formatDelay } from '@/lib/format'
import { cn } from '@/lib/cn'

const ranges: { value: TimeRange; labelKey: string }[] = [
  { value: '15m', labelKey: 'trends.fifteenMinutes' },
  { value: '1h', labelKey: 'trends.oneHour' },
  { value: '6h', labelKey: 'trends.sixHours' },
  { value: '24h', labelKey: 'trends.twentyFourHours' },
  { value: '7d', labelKey: 'trends.sevenDays' },
]

type TrendDimension = 'global' | 'network' | 'inbound' | 'outbound'

export function TrendsPage() {
  const { t, i18n } = useTranslation()
  const locale = i18n.language
  const [range, setRange] = useState<TimeRange>('1h')
  const [dimension, setDimension] = useState<TrendDimension>('global')
  const [value, setValue] = useState('')
  const overview = useOverview(range)
  const rankings = useRankings({
    dimension: dimension === 'global' ? 'outbound' : dimension,
    sort: 'traffic',
    range,
    limit: 50,
    enabled: dimension !== 'global',
  })
  const dimensionSeries = useDimensionSeries({
    dimension: dimension === 'global' ? 'outbound' : dimension,
    value,
    range,
  })

  useEffect(() => {
    if (dimension === 'global') {
      setValue('')
      return
    }
    const values = rankings.data?.data.map((item) => item.value) ?? []
    if (values.length > 0 && !values.includes(value)) setValue(values[0])
  }, [dimension, rankings.data, value])

  const data = overview.data
  const selectedSeries = dimension === 'global' ? data?.series ?? [] : dimensionSeries.data?.series ?? []
  const maximumMemory = dimension === 'global'
    ? Math.max(0, ...selectedSeries.map((point) => 'memoryBytes' in point ? point.memoryBytes ?? 0 : 0))
    : 0
  const memoryUnit = byteUnit(maximumMemory)
  const loading = overview.isLoading || (dimension !== 'global' && (rankings.isLoading || dimensionSeries.isLoading))
  const error = overview.error || rankings.error || dimensionSeries.error

  return (
    <div>
      <PageHeader
        title={t('trends.title')}
        actions={
        <Select value={range} onValueChange={(next) => setRange(next as TimeRange)}>
          <SelectTrigger className="w-28"><SelectValue /></SelectTrigger>
          <SelectContent>{ranges.map((item) => <SelectItem key={item.value} value={item.value}>{t(item.labelKey)}</SelectItem>)}</SelectContent>
        </Select>
        }
      />

      <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <Tabs value={dimension} onValueChange={(next) => setDimension(next as TrendDimension)}>
          <TabsList className="grid w-full grid-cols-4 sm:w-auto">
            <TabsTrigger value="global">{t('trends.global')}</TabsTrigger>
            <TabsTrigger value="outbound">{t('connections.outbound')}</TabsTrigger>
            <TabsTrigger value="inbound">{t('connections.inbound')}</TabsTrigger>
            <TabsTrigger value="network">{t('connections.network')}</TabsTrigger>
          </TabsList>
        </Tabs>
        {dimension !== 'global' && rankings.data && rankings.data.data.length > 0 && (
          <Select value={value} onValueChange={setValue}>
            <SelectTrigger className="w-full sm:w-56"><SelectValue /></SelectTrigger>
            <SelectContent>{rankings.data.data.map((item) => <SelectItem key={item.value} value={item.value}>{item.value}</SelectItem>)}</SelectContent>
          </Select>
        )}
      </div>

      {loading && <div className="space-y-3"><ChartSkeleton /><ChartSkeleton /></div>}
      {error && !data && <ErrorState error={error as Error} onRetry={() => overview.refetch()} />}

      {!loading && data && (
        <>
          {dimension === 'global' && data.rangeTotals && (
            <div className="grid grid-cols-2 gap-2.5 sm:grid-cols-3 sm:gap-3 max-[360px]:grid-cols-1">
              <SummaryMetric icon={ArrowDown} label={t('common.download')} value={formatBytesCompact(data.rangeTotals.downloadBytes)} />
              <SummaryMetric icon={ArrowUp} label={t('common.upload')} value={formatBytesCompact(data.rangeTotals.uploadBytes)} />
              <SummaryMetric className="col-span-2 sm:col-span-1 max-[360px]:col-span-1" icon={Activity} label={t('common.connections')} value={formatCount(data.rangeTotals.connections, locale)} />
            </div>
          )}

          <Card>
            <CardHeader className="flex-row items-center justify-between">
              <CardTitle>{dimension === 'global' ? t('trends.traffic') : value || t('common.empty')}</CardTitle>
              {dimension !== 'global' && <span className="font-mono text-[11px] text-[var(--color-text-faint)]">{t(`trends.${dimension}`)}</span>}
            </CardHeader>
            <CardContent>
              {selectedSeries.length > 0 ? <TrafficChart data={selectedSeries} height={200} /> : <EmptyChart />}
            </CardContent>
          </Card>

          <div className="grid gap-3 lg:grid-cols-2">
            <Card>
              <CardHeader><CardTitle>{t('trends.connectionsChart')}</CardTitle></CardHeader>
              <CardContent>
                {selectedSeries.length > 0 ? (
                  <MultiLineChart
                    data={selectedSeries}
                    height={160}
                    series={[{ key: 'activeConnections', color: 'var(--color-healthy)', name: t('common.active') }]}
                    yTickFormatter={(number) => axisNumber(number, locale)}
                  />
                ) : <EmptyChart compact />}
              </CardContent>
            </Card>

            <Card>
              <CardHeader><CardTitle>{dimension === 'global' ? t('trends.runtime') : t('trends.quality')}</CardTitle></CardHeader>
              <CardContent>
                {selectedSeries.length > 0 ? (
                  <MultiLineChart
                    data={selectedSeries}
                    height={160}
                    series={dimension === 'global'
                      ? [{ key: 'memoryBytes', color: 'var(--color-upload)', name: t('overview.memory'), formatter: (number) => number == null ? '—' : `${axisNumber(number, locale, true)} ${memoryUnit.label}` }]
                      : [{ key: 'delayMs', color: 'var(--color-download)', name: t('trends.delay'), formatter: (number) => number == null ? '—' : formatDelay(number, locale) }]}
                    valueScale={dimension === 'global' ? memoryUnit.factor : 1}
                    unit={dimension === 'global' ? memoryUnit.label : 'ms'}
                    yTickFormatter={(number) => axisNumber(number, locale)}
                  />
                ) : <EmptyChart compact />}
              </CardContent>
            </Card>
          </div>
        </>
      )}
      </div>
    </div>
  )
}

function SummaryMetric({ className, icon: Icon, label, value }: { className?: string; icon: React.ElementType; label: string; value: string }) {
  return (
    <div className={cn('surface-card min-w-0 p-3.5 lg:p-5', className)} data-summary-metric>
      <div className="mb-2.5 flex items-center gap-2 text-[13px] font-semibold"><Icon className="h-4 w-4 text-[var(--color-text-faint)]" strokeWidth={1.8} />{label}</div>
      <div className="break-words text-[20px] font-semibold leading-tight tabular-nums tracking-[-0.01em] sm:text-[22px]" data-summary-value title={value}>{value}</div>
    </div>
  )
}

function EmptyChart({ compact = false }: { compact?: boolean }) {
  const { t } = useTranslation()
  return <div className={`flex items-center justify-center text-xs text-[var(--color-text-faint)] ${compact ? 'h-40' : 'h-[200px]'}`}><Database className="mr-2 h-4 w-4" />{t('common.empty')}</div>
}

function byteUnit(maximum: number) {
  const units = [
    { factor: 1, label: 'B' },
    { factor: 1024, label: 'KiB' },
    { factor: 1024 ** 2, label: 'MiB' },
    { factor: 1024 ** 3, label: 'GiB' },
  ]
  let selected = units[0]
  for (const candidate of units) {
    if (maximum >= candidate.factor) selected = candidate
  }
  return selected
}

function axisNumber(value: number, locale: string, detailed = false) {
  return value.toLocaleString(locale, { maximumFractionDigits: detailed ? 2 : value < 10 ? 1 : 0 })
}
