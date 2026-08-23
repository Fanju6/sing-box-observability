import { describe, it, expect } from 'vitest'
import { formatBytes, formatBytesCompact, formatByteRate, formatDuration, formatCount, formatRelativeTime, formatLocalDateTime, formatPercent, formatDelay } from '@/lib/format'

describe('format utilities', () => {
  it('formatBytes handles different sizes', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(1024, 1)).toBe('1 KiB')
    expect(formatBytes(1048576, 1)).toBe('1 MiB')
    expect(formatBytes(1073741824, 1)).toBe('1 GiB')
    expect(formatBytes(null)).toBe('—')
  })

  it('formatBytesCompact keeps metric values short without hiding their unit', () => {
    expect(formatBytesCompact(1.81 * 1024 ** 3)).toBe('1.81 GiB')
    expect(formatBytesCompact(90 * 1024 ** 2)).toBe('90 MiB')
    expect(formatBytesCompact(999.9 * 1024 ** 3)).toBe('1000 GiB')
  })

  it('formatByteRate returns rates with /s', () => {
    expect(formatByteRate(0)).toBe('0 B/s')
    expect(formatByteRate(1024, 1)).toBe('1 KiB/s')
  })

  it('formatDuration handles different durations', () => {
    expect(formatDuration(30)).toBe('30s')
    expect(formatDuration(60)).toBe('1m')
    expect(formatDuration(3600)).toBe('1h')
    expect(formatDuration(86400)).toBe('1d')
  })

  it('formatCount returns locale-formatted number', () => {
    expect(formatCount(1000, 'en')).toBe('1,000')
    expect(formatCount(null, 'en')).toBe('—')
  })

  it('formatPercent works', () => {
    expect(formatPercent(50.5)).toBe('50.5%')
  })

  it('formatDelay works', () => {
    expect(formatDelay(48, 'en')).toBe('48ms')
  })

  it('formatRelativeTime returns string', () => {
    const result = formatRelativeTime(new Date(Date.now() - 60000).toISOString(), 'en')
    expect(result).toBeTruthy()
  })

  it('formatLocalDateTime returns string', () => {
    const result = formatLocalDateTime(new Date().toISOString(), 'en')
    expect(result).toBeTruthy()
  })
})
