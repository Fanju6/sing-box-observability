/**
 * TrafficSparkline is adapted from sing-box-dashboard.
 * Copyright (C) 2022 nekohasekai <contact-sagernet@sekai.icu>.
 * Modifications Copyright (C) 2026 Fanju and contributors.
 * See NOTICE and THIRD_PARTY_LICENSES.txt.
 */
import { useTranslation } from 'react-i18next'
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { formatLocalTime } from '@/lib/format'

interface TrafficPoint {
  timestamp: string
  downloadBytesPerSecond?: number | null
  uploadBytesPerSecond?: number | null
}

interface TrafficChartProps {
  data: TrafficPoint[]
  height?: number
}

export function TrafficChart({ data, height = 200 }: TrafficChartProps) {
  const { t, i18n } = useTranslation()
  const locale = i18n.language
  const maximum = Math.max(0, ...data.flatMap((point) => [point.downloadBytesPerSecond ?? 0, point.uploadBytesPerSecond ?? 0]))
  const unit = byteRateUnit(maximum)

  const chartData = data.map((p) => ({
    time: new Date(p.timestamp).getTime(),
    download: p.downloadBytesPerSecond == null ? null : p.downloadBytesPerSecond / unit.factor,
    upload: p.uploadBytesPerSecond == null ? null : p.uploadBytesPerSecond / unit.factor,
    timeLabel: formatLocalTime(p.timestamp, locale),
  }))

  return (
    <div className="relative min-w-0" style={{ height }}>
      <span className="pointer-events-none absolute right-1 top-0 z-10 font-mono text-[10px] text-[var(--color-text-faint)]">{unit.label}</span>
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart data={chartData} margin={{ top: 12, right: 4, left: -10, bottom: 0 }}>
        <defs>
          <linearGradient id="downloadGrad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--color-download)" stopOpacity={0.3} />
            <stop offset="100%" stopColor="var(--color-download)" stopOpacity={0} />
          </linearGradient>
          <linearGradient id="uploadGrad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--color-upload)" stopOpacity={0.3} />
            <stop offset="100%" stopColor="var(--color-upload)" stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" vertical={false} />
        <XAxis
          dataKey="time"
          type="number"
          scale="time"
          domain={['dataMin', 'dataMax']}
          tickFormatter={(v) => formatLocalTime(new Date(v).toISOString(), locale)}
          tick={{ fontSize: 11, fill: 'var(--color-text-faint)' }}
          axisLine={{ stroke: 'var(--color-border)' }}
          tickLine={false}
          minTickGap={48}
        />
        <YAxis
          tickFormatter={(value) => compactNumber(Number(value), locale)}
          tick={{ fontSize: 11, fill: 'var(--color-text-faint)' }}
          axisLine={false}
          tickLine={false}
          width={38}
        />
        <Tooltip
          contentStyle={{
            backgroundColor: 'var(--color-surface-1)',
            border: '1px solid var(--color-border)',
            borderRadius: 8,
            fontSize: 12,
          }}
          labelFormatter={(v) => new Date(Number(v)).toLocaleString(locale)}
          formatter={(value, name) => {
            const numValue = value as number | null
            if (numValue == null) return ['—', String(name)]
            return [`${compactNumber(numValue, locale, true)} ${unit.label}`, name === 'download' ? t('common.download') : t('common.upload')]
          }}
        />
        <Area
          type="monotone"
          dataKey="download"
          stroke="var(--color-download)"
          fill="url(#downloadGrad)"
          strokeWidth={2}
          connectNulls={false}
          isAnimationActive={false}
        />
        <Area
          type="monotone"
          dataKey="upload"
          stroke="var(--color-upload)"
          fill="url(#uploadGrad)"
          strokeWidth={2}
          connectNulls={false}
          isAnimationActive={false}
        />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  )
}

export function TrafficSparkline({
  data,
  dataKey,
  color = 'var(--color-primary)',
  height = 46,
}: TrafficChartProps & { dataKey: 'downloadBytesPerSecond' | 'uploadBytesPerSecond'; color?: string }) {
  const width = 300
  const capacity = 30
  const values = data.slice(-capacity).map((point) => point[dataKey] ?? 0)
  const maximum = Math.max(...values, 1)
  const stepX = width / Math.max(capacity - 1, 1)
  const offset = Math.max(0, capacity - values.length)
  const points = values.map((value, index) => {
    const x = (offset + index) * stepX
    const y = height - 3 - value / (maximum * 1.2) * (height - 6)
    return `${x.toFixed(1)},${y.toFixed(1)}`
  })

  return (
    <svg width="100%" height={height} viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" className="block" aria-hidden="true">
      {points.length > 1 && (
        <>
          <polygon points={`${points[0].split(',')[0]},${height} ${points.join(' ')} ${points.at(-1)?.split(',')[0]},${height}`} fill={color} opacity="0.1" />
          <polyline points={points.join(' ')} fill="none" stroke={color} strokeWidth="1.8" strokeLinejoin="round" strokeLinecap="round" vectorEffect="non-scaling-stroke" />
        </>
      )}
    </svg>
  )
}

function byteRateUnit(maximum: number) {
  const units = [
    { factor: 1, label: 'B/s' },
    { factor: 1024, label: 'KiB/s' },
    { factor: 1024 ** 2, label: 'MiB/s' },
    { factor: 1024 ** 3, label: 'GiB/s' },
  ]
  let selected = units[0]
  for (const candidate of units) {
    if (maximum >= candidate.factor) selected = candidate
  }
  return selected
}

function compactNumber(value: number, locale: string, detailed = false) {
  return value.toLocaleString(locale, {
    maximumFractionDigits: detailed ? 2 : value < 10 ? 1 : 0,
  })
}
