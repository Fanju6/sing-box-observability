import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { TimeRange, TimeWindowSelection } from '@/api/types'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { formatLocalDateTime } from '@/lib/format'
import {
  availableTimeRangePresets,
  earliestAvailableTime,
  fromDateTimeLocalValue,
  presetDurationSeconds,
  toDateTimeLocalValue,
} from '@/lib/time-window'
import { cn } from '@/lib/cn'

interface TimeRangePickerProps {
  value: TimeWindowSelection
  onChange: (value: TimeWindowSelection) => void
  retentionSeconds?: number
  historyAvailableFrom?: string | null
  className?: string
}

interface DateBounds {
  minimum: Date
  maximum: Date
}

export function TimeRangePicker({
  value,
  onChange,
  retentionSeconds,
  historyAvailableFrom,
  className,
}: TimeRangePickerProps) {
  const { t, i18n } = useTranslation()
  const [customOpen, setCustomOpen] = useState(false)
  const [fromValue, setFromValue] = useState('')
  const [toValue, setToValue] = useState('')
  const [bounds, setBounds] = useState<DateBounds>(() => createBounds(retentionSeconds, historyAvailableFrom))
  const presets = availableTimeRangePresets(retentionSeconds)
  const selectValue = 'from' in value ? 'custom' : value.range

  const openCustom = () => {
    const nextBounds = createBounds(retentionSeconds, historyAvailableFrom)
    const selectedTo = 'from' in value ? new Date(value.to) : nextBounds.maximum
    const selectedFrom = 'from' in value
      ? new Date(value.from)
      : new Date(selectedTo.getTime() - presetDurationSeconds(value.range) * 1000)
    setBounds(nextBounds)
    setFromValue(toDateTimeLocalValue(clampDate(selectedFrom, nextBounds.minimum, nextBounds.maximum)))
    setToValue(toDateTimeLocalValue(clampDate(selectedTo, nextBounds.minimum, nextBounds.maximum)))
    setCustomOpen(true)
  }

  const handleSelect = (next: string) => {
    if (next === 'custom') {
      openCustom()
      return
    }
    onChange({ range: next as TimeRange })
  }

  const fromDate = fromDateTimeLocalValue(fromValue)
  const toDate = fromDateTimeLocalValue(toValue)
  const error = validateCustomRange(fromDate, toDate, bounds, t)

  const applyCustomRange = () => {
    if (!fromDate || !toDate || error) return
    onChange({ from: fromDate.toISOString(), to: toDate.toISOString() })
    setCustomOpen(false)
  }

  return (
    <>
      <Select value={selectValue} onValueChange={handleSelect}>
        <SelectTrigger className={cn('w-28', className)} aria-label={t('timeRange.label')}>
          <SelectValue>{selectValue === 'custom' ? t('timeRange.custom') : undefined}</SelectValue>
        </SelectTrigger>
        <SelectContent>
          {presets.map((preset) => (
            <SelectItem key={preset.value} value={preset.value}>{t(preset.labelKey)}</SelectItem>
          ))}
          <SelectItem value="custom">{t('timeRange.custom')}</SelectItem>
        </SelectContent>
      </Select>

      <Dialog open={customOpen} onOpenChange={setCustomOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className="font-serif">{t('timeRange.customTitle')}</DialogTitle>
            <DialogDescription>
              {t('timeRange.availableSince', {
                date: formatLocalDateTime(bounds.minimum.toISOString(), i18n.language),
              })}
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 sm:grid-cols-2">
            <label className="grid gap-1.5 text-[13px] font-medium">
              <span>{t('timeRange.from')}</span>
              <Input
                type="datetime-local"
                value={fromValue}
                min={toDateTimeLocalValue(bounds.minimum)}
                max={toValue || toDateTimeLocalValue(bounds.maximum)}
                onChange={(event) => setFromValue(event.target.value)}
              />
            </label>
            <label className="grid gap-1.5 text-[13px] font-medium">
              <span>{t('timeRange.to')}</span>
              <Input
                type="datetime-local"
                value={toValue}
                min={fromValue || toDateTimeLocalValue(bounds.minimum)}
                max={toDateTimeLocalValue(bounds.maximum)}
                onChange={(event) => setToValue(event.target.value)}
              />
            </label>
          </div>

          <div className="min-h-5 text-xs text-[var(--color-danger)]" role="alert">
            {error ?? ''}
          </div>

          <DialogFooter className="flex-row items-center justify-end gap-2 space-x-0">
            <Button type="button" variant="secondary" onClick={() => setCustomOpen(false)}>
              {t('common.cancel')}
            </Button>
            <Button type="button" onClick={applyCustomRange} disabled={Boolean(error)}>
              {t('timeRange.apply')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

function createBounds(retentionSeconds?: number, historyAvailableFrom?: string | null): DateBounds {
  const now = new Date()
  return {
    minimum: roundToMinute(earliestAvailableTime(retentionSeconds, historyAvailableFrom, now), 'ceil'),
    maximum: roundToMinute(now, 'floor'),
  }
}

function roundToMinute(date: Date, direction: 'ceil' | 'floor') {
  const operation = direction === 'ceil' ? Math.ceil : Math.floor
  return new Date(operation(date.getTime() / 60_000) * 60_000)
}

function clampDate(date: Date, minimum: Date, maximum: Date) {
  if (Number.isNaN(date.getTime())) return maximum
  return new Date(Math.min(maximum.getTime(), Math.max(minimum.getTime(), date.getTime())))
}

function validateCustomRange(
  from: Date | null,
  to: Date | null,
  bounds: DateBounds,
  t: (key: string) => string,
) {
  if (!from || !to) return t('timeRange.required')
  if (to.getTime() <= from.getTime()) return t('timeRange.invalidOrder')
  if (from.getTime() < bounds.minimum.getTime() || to.getTime() > bounds.maximum.getTime()) {
    return t('timeRange.outsideRetention')
  }
  return null
}
