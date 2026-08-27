import { describe, expect, it } from 'vitest'
import {
  availableTimeRangePresets,
  earliestAvailableTime,
  timeWindowKey,
  timeWindowParams,
} from '@/lib/time-window'

describe('time window helpers', () => {
  it('only exposes presets within retention', () => {
    expect(availableTimeRangePresets(7 * 24 * 60 * 60).map((item) => item.value))
      .toEqual(['15m', '1h', '6h', '24h', '7d'])
    expect(availableTimeRangePresets(30 * 24 * 60 * 60).map((item) => item.value))
      .toContain('30d')
  })

  it('uses the later storage boundary as the custom range minimum', () => {
    const now = new Date('2026-08-27T12:00:00Z')
    const actual = earliestAvailableTime(30 * 24 * 60 * 60, '2026-08-20T00:00:00Z', now)
    expect(actual.toISOString()).toBe('2026-08-20T00:00:00.000Z')
  })

  it('builds stable preset and custom query values', () => {
    expect(timeWindowParams({ range: '7d' })).toEqual({ range: '7d' })
    expect(timeWindowKey({ from: '2026-08-01T00:00:00Z', to: '2026-08-02T00:00:00Z' }))
      .toBe('2026-08-01T00:00:00Z/2026-08-02T00:00:00Z')
  })
})
