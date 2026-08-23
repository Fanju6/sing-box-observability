import { useTranslation } from 'react-i18next'
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { DimensionTimePoint, TimePoint } from '@/api/types'
import { formatCount, formatLocalTime } from '@/lib/format'

interface LineSeriesConfig {
  key: keyof TimePoint | keyof DimensionTimePoint | string
  color: string
  name: string
  formatter?: (v: number | null) => string
}

interface MultiLineChartProps {
  data: Array<TimePoint | DimensionTimePoint>
  series: LineSeriesConfig[]
  height?: number
  yTickFormatter?: (v: number) => string
  unit?: string
  valueScale?: number
}

export function MultiLineChart({ data, series, height = 200, yTickFormatter, unit, valueScale = 1 }: MultiLineChartProps) {
  const { i18n } = useTranslation()
  const locale = i18n.language

  const chartData = data.map((point) => {
    const scaled: Record<string, unknown> = { time: new Date(point.timestamp).getTime(), ...point }
    if (valueScale !== 1) {
      for (const item of series) {
        const key = String(item.key)
        if (typeof scaled[key] === 'number') scaled[key] = (scaled[key] as number) / valueScale
      }
    }
    return scaled
  })

  return (
    <div className="relative min-w-0" style={{ height }}>
      {unit && <span className="pointer-events-none absolute right-1 top-0 z-10 font-mono text-[10px] text-[var(--color-text-faint)]">{unit}</span>}
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={chartData} margin={{ top: 12, right: 4, left: -10, bottom: 0 }}>
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
          tickFormatter={yTickFormatter}
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
            const cfg = series.find((s) => s.key === name)
            const numValue = value as number | null
            if (numValue == null) return ['—', cfg?.name || String(name)]
            if (cfg?.formatter) return [cfg.formatter(numValue), cfg.name]
            return [formatCount(numValue, locale), cfg?.name || String(name)]
          }}
        />
        {series.map((s) => (
          <Line
            key={String(s.key)}
            type="monotone"
            dataKey={String(s.key)}
            stroke={s.color}
            strokeWidth={2}
            dot={false}
            connectNulls={false}
            isAnimationActive={false}
          />
        ))}
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
