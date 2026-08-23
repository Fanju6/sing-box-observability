import { useTranslation } from 'react-i18next'
import type { RankingItem } from '@/api/types'
import { formatBytes } from '@/lib/format'

interface RankingsBarChartProps {
  data: RankingItem[]
  sortKey: 'traffic' | 'connections' | 'download' | 'upload'
}

export function RankingsBarChart({ data, sortKey }: RankingsBarChartProps) {
  const { i18n } = useTranslation()
  const locale = i18n.language

  const getValue = (item: RankingItem): number => {
    switch (sortKey) {
      case 'traffic':
        return item.downloadBytes + item.uploadBytes
      case 'connections':
        return item.connections
      case 'download':
        return item.downloadBytes
      case 'upload':
        return item.uploadBytes
    }
  }

  const chartData = data.map((item) => ({
      name: item.value,
      value: getValue(item),
      download: item.downloadBytes,
      upload: item.uploadBytes,
      connections: item.connections,
      percentage: item.percentage,
    }))

  const formatValue = (v: number): string => {
    if (sortKey === 'connections') return v.toLocaleString(locale)
    return formatBytes(v, 1)
  }

  const maxVal = Math.max(...chartData.map((d) => d.value), 1)

  return (
    <div className="space-y-2">
      {chartData.map((item) => (
        <div key={item.name} className="flex items-center gap-3">
          <div className="w-28 sm:w-40 truncate text-xs font-mono text-[var(--color-text-muted)]" title={item.name}>
            {item.name}
          </div>
          <div className="relative h-7 flex-1 overflow-hidden rounded-md bg-[var(--color-inset)]">
            <div
              className="h-full rounded-md bg-[var(--color-primary)] opacity-50 transition-all"
              style={{ width: `${(item.value / maxVal) * 100}%` }}
            />
            <div className="absolute inset-0 flex items-center justify-between px-2 text-xs">
              <span className="text-[var(--color-text)] font-medium">{item.percentage.toFixed(1)}%</span>
              <span className="text-[var(--color-text-muted)] font-mono tabular-nums">{formatValue(item.value)}</span>
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}
