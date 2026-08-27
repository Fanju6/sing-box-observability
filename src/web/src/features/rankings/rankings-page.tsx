import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useRankings, useMeta } from '@/api/hooks'
import type { RankingDimension, RankingSort, TimeWindowSelection } from '@/api/types'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Badge } from '@/components/ui/badge'
import { RankingsBarChart } from '@/components/charts/rankings-bar'
import { EmptyState, ErrorState, CardSkeleton } from '@/components/data-state/states'
import { PageHeader } from '@/components/layout/page-header'
import { TimeRangePicker } from '@/components/time-range-picker'
import { formatBytes, formatCount } from '@/lib/format'
import { Lock } from 'lucide-react'

const sortOptions: { value: RankingSort; labelKey: string }[] = [
  { value: 'traffic', labelKey: 'rankings.sortTraffic' },
  { value: 'connections', labelKey: 'rankings.sortConnections' },
  { value: 'download', labelKey: 'rankings.sortDownload' },
  { value: 'upload', labelKey: 'rankings.sortUpload' },
]

const dimensionLabelKeys: Record<RankingDimension, string> = {
  network: 'rankings.dimNetwork',
  inbound: 'rankings.dimInbound',
  outbound: 'rankings.dimOutbound',
  rule: 'rankings.dimRule',
  domain: 'rankings.dimDomain',
  destination_ip: 'rankings.dimDestIp',
  source: 'rankings.dimSource',
  process: 'rankings.dimProcess',
  user: 'rankings.dimUser',
}

const baseDimensions: RankingDimension[] = ['network', 'inbound', 'outbound']
const allDimensions = Object.keys(dimensionLabelKeys) as RankingDimension[]

export function RankingsPage() {
  const { t, i18n } = useTranslation()
  const locale = i18n.language
  const { data: meta } = useMeta()
  const [dimension, setDimension] = useState<RankingDimension>('outbound')
  const [sort, setSort] = useState<RankingSort>('traffic')
  const [window, setWindow] = useState<TimeWindowSelection>({ range: '1h' })

  const availableDimensions = meta?.capabilities?.rankingDimensions ?? baseDimensions
  const sensitiveDimensions = meta?.capabilities?.sensitiveDimensions ?? allDimensions.filter((item) => !baseDimensions.includes(item))
  const isSensitiveDisabled = meta?.capabilities != null && !meta.capabilities.exposeSensitive && sensitiveDimensions.includes(dimension)

  const { data, isLoading, error, refetch } = useRankings({
    dimension,
    sort,
    window,
    limit: 20,
    enabled: !isSensitiveDisabled,
  })

  const isSensitiveError = error && 'code' in error && (error as { code: string }).code === 'SENSITIVE_DIMENSION_DISABLED'

  return (
    <div className="min-w-0">
      <PageHeader
        title={t('rankings.title')}
        actions={
        <div className="flex gap-2">
          <Select value={dimension} onValueChange={(next) => setDimension(next as RankingDimension)}>
            <SelectTrigger className="w-36"><SelectValue /></SelectTrigger>
            <SelectContent>
              {allDimensions.map((item) => {
                const sensitive = sensitiveDimensions.includes(item)
                const available = availableDimensions.includes(item) && (meta?.capabilities?.exposeSensitive || !sensitive)
                return <SelectItem key={item} value={item} disabled={!available}>{t(dimensionLabelKeys[item])}{!available ? ` · ${t('settings.disabled')}` : ''}</SelectItem>
              })}
            </SelectContent>
          </Select>
          <TimeRangePicker
            value={window}
            onChange={setWindow}
            retentionSeconds={meta?.collector.retentionSeconds}
            historyAvailableFrom={meta?.source.historyAvailableFrom}
          />
        </div>
        }
      />

      <Card>
        <CardHeader className="pb-3">
          <CardTitle>{t(dimensionLabelKeys[dimension])}</CardTitle>
          <div className="mt-2 overflow-x-auto pb-1">
            <Tabs value={sort} onValueChange={(v) => setSort(v as RankingSort)}>
              <TabsList className="min-w-max">
                {sortOptions.map((s) => (
                  <TabsTrigger key={s.value} value={s.value}>
                    {t(s.labelKey)}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>
          </div>
        </CardHeader>
        <CardContent>
          {isLoading && (
            <div className="space-y-3 py-4">
              {Array.from({ length: 6 }).map((_, i) => (
                <div key={i} className="flex items-center gap-3">
                  <CardSkeleton className="h-6 flex-1" />
                </div>
              ))}
            </div>
          )}

          {isSensitiveDisabled && (
            <EmptyState
              icon={<Lock className="h-7 w-7" />}
              title={t('rankings.sensitiveDisabled')}
              description={t('rankings.sensitiveDescription')}
            />
          )}

          {error && !isSensitiveError && !isLoading && <ErrorState error={error as Error} onRetry={refetch} />}

          {data && data.data.length === 0 && !isLoading && !isSensitiveError && !isSensitiveDisabled && (
            <EmptyState />
          )}

          {data && data.data.length > 0 && !isSensitiveDisabled && (
            <div className="space-y-4">
              <div className="hidden lg:block"><RankingsBarChart data={data.data} sortKey={sort} /></div>

              <div className="hidden lg:block overflow-x-auto">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="border-b border-[var(--color-border)] text-[var(--color-text-faint)]">
                      <th className="text-left py-2 w-8">#</th>
                      <th className="text-left py-2">{t('rankings.dimension')}</th>
                      <th className="text-right py-2">{t('rankings.percentage')}</th>
                      <th className="text-right py-2">{t('common.download')}</th>
                      <th className="text-right py-2">{t('common.upload')}</th>
                      <th className="text-right py-2">{t('common.connections')}</th>
                      <th className="text-right py-2">{t('common.active')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.data.map((item, i) => (
                      <tr key={item.value} className="border-b border-[var(--color-border)] last:border-0">
                        <td className="py-2 text-[var(--color-text-faint)] font-mono">{i + 1}</td>
                        <td className="py-2 font-mono truncate max-w-[200px]" title={item.value}>{item.value}</td>
                        <td className="py-2 text-right tabular-nums">
                          <Badge variant={i < 3 ? 'primary' : 'default'} className="text-[10px]">
                            {item.percentage.toFixed(1)}%
                          </Badge>
                        </td>
                        <td className="py-2 text-right tabular-nums font-mono">
                          {formatBytes(item.downloadBytes, 1)}
                        </td>
                        <td className="py-2 text-right tabular-nums font-mono">
                          {formatBytes(item.uploadBytes, 1)}
                        </td>
                        <td className="py-2 text-right tabular-nums font-mono">
                          {formatCount(item.connections, locale)}
                        </td>
                        <td className="py-2 text-right tabular-nums text-[var(--color-text-muted)] font-mono">
                          {formatCount(item.activeConnections, locale)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              <div className="min-w-0 divide-y divide-[var(--color-border)] lg:hidden">
                {data.data.map((item, i) => (
                  <div key={item.value} className="min-w-0 overflow-hidden py-3 first:pt-0 last:pb-0">
                    <div className="flex items-center justify-between gap-2 mb-2 min-w-0">
                      <div className="flex items-center gap-2 min-w-0 flex-1">
                        <span className="text-xs font-mono text-[var(--color-text-faint)] w-5 flex-shrink-0">{i + 1}</span>
                        <span className="font-mono text-sm truncate min-w-0 flex-1" title={item.value}>{item.value}</span>
                      </div>
                      <Badge variant={i < 3 ? 'primary' : 'default'} className="text-[10px] flex-shrink-0">
                        {item.percentage.toFixed(1)}%
                      </Badge>
                    </div>
                    <div className="grid grid-cols-2 gap-2 text-xs">
                      <div>
                        <span className="text-[var(--color-text-faint)]">↓ </span>
                        <span className="font-mono tabular-nums">
                          {formatBytes(item.downloadBytes, 1)}
                        </span>
                      </div>
                      <div>
                        <span className="text-[var(--color-text-faint)]">↑ </span>
                        <span className="font-mono tabular-nums">
                          {formatBytes(item.uploadBytes, 1)}
                        </span>
                      </div>
                      <div>
                        <span className="text-[var(--color-text-faint)]">{t('common.connections')} </span>
                        <span className="font-mono tabular-nums">{formatCount(item.connections, locale)}</span>
                      </div>
                      <div>
                        <span className="text-[var(--color-text-faint)]">{t('common.active')} </span>
                        <span className="font-mono tabular-nums">{formatCount(item.activeConnections, locale)}</span>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
