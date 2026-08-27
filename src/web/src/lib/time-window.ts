import type { TimeRange, TimeWindowSelection } from '@/api/types'

export const DEFAULT_RETENTION_SECONDS = 7 * 24 * 60 * 60

export const TIME_RANGE_PRESETS: ReadonlyArray<{
  value: TimeRange
  seconds: number
  labelKey: string
}> = [
  { value: '15m', seconds: 15 * 60, labelKey: 'timeRange.fifteenMinutes' },
  { value: '1h', seconds: 60 * 60, labelKey: 'timeRange.oneHour' },
  { value: '6h', seconds: 6 * 60 * 60, labelKey: 'timeRange.sixHours' },
  { value: '24h', seconds: 24 * 60 * 60, labelKey: 'timeRange.twentyFourHours' },
  { value: '7d', seconds: 7 * 24 * 60 * 60, labelKey: 'timeRange.sevenDays' },
  { value: '30d', seconds: 30 * 24 * 60 * 60, labelKey: 'timeRange.thirtyDays' },
  { value: '90d', seconds: 90 * 24 * 60 * 60, labelKey: 'timeRange.ninetyDays' },
]

export function availableTimeRangePresets(retentionSeconds?: number) {
  const retention = retentionSeconds && retentionSeconds > 0
    ? retentionSeconds
    : DEFAULT_RETENTION_SECONDS
  return TIME_RANGE_PRESETS.filter((preset) => preset.seconds <= retention)
}

export function timeWindowParams(selection: TimeWindowSelection) {
  return 'from' in selection
    ? { from: selection.from, to: selection.to }
    : { range: selection.range }
}

export function timeWindowKey(selection: TimeWindowSelection) {
  return 'from' in selection
    ? `${selection.from}/${selection.to}`
    : selection.range
}

export function earliestAvailableTime(
  retentionSeconds?: number,
  historyAvailableFrom?: string | null,
  now = new Date(),
) {
  const retention = retentionSeconds && retentionSeconds > 0
    ? retentionSeconds
    : DEFAULT_RETENTION_SECONDS
  const retentionStart = now.getTime() - retention * 1000
  const historyStart = historyAvailableFrom ? Date.parse(historyAvailableFrom) : Number.NaN
  return new Date(Number.isFinite(historyStart) ? Math.max(retentionStart, historyStart) : retentionStart)
}

export function toDateTimeLocalValue(date: Date) {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 16)
}

export function fromDateTimeLocalValue(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date
}

export function presetDurationSeconds(range: TimeRange) {
  return TIME_RANGE_PRESETS.find((preset) => preset.value === range)?.seconds ?? 60 * 60
}
