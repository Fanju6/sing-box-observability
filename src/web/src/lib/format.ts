const IEC_UNITS = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB'] as const

export function formatBytes(bytes: number | null | undefined, decimals = 2): string {
  if (bytes == null) return '—'
  if (bytes === 0) return '0 B'
  const k = 1024
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), IEC_UNITS.length - 1)
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(decimals))} ${IEC_UNITS[i]}`
}

export function formatBytesCompact(bytes: number | null | undefined): string {
  if (bytes == null) return '—'
  if (bytes === 0) return '0 B'
  const k = 1024
  const magnitude = Math.abs(bytes)
  const i = Math.min(Math.floor(Math.log(magnitude) / Math.log(k)), IEC_UNITS.length - 1)
  const scaled = bytes / Math.pow(k, i)
  const decimals = Math.abs(scaled) >= 100 ? 0 : Math.abs(scaled) >= 10 ? 1 : 2
  return `${Number(scaled.toFixed(decimals))} ${IEC_UNITS[i]}`
}

export function formatByteRate(bytesPerSecond: number | null | undefined, decimals = 2): string {
  if (bytesPerSecond == null) return '—'
  return `${formatBytes(bytesPerSecond, decimals)}/s`
}

export function formatDuration(seconds: number): string {
  if (seconds < 60) return `${Math.floor(seconds)}s`
  if (seconds < 3600) {
    const m = Math.floor(seconds / 60)
    const s = Math.floor(seconds % 60)
    return `${m}m${s > 0 ? ` ${s}s` : ''}`
  }
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (seconds < 86400) return `${h}h${m > 0 ? ` ${m}m` : ''}`
  const d = Math.floor(seconds / 86400)
  const rh = Math.floor((seconds % 86400) / 3600)
  return `${d}d${rh > 0 ? ` ${rh}h` : ''}`
}

export function formatRelativeTime(isoString: string | null | undefined, locale: string): string {
  if (!isoString) return '—'
  const date = new Date(isoString)
  const now = Date.now()
  const diffMs = now - date.getTime()
  const diffSec = Math.floor(diffMs / 1000)
  if (diffSec < 60) return locale.startsWith('zh') ? '刚刚' : 'just now'
  if (diffSec < 3600) {
    const m = Math.floor(diffSec / 60)
    return locale.startsWith('zh') ? `${m} 分钟前` : `${m}m ago`
  }
  if (diffSec < 86400) {
    const h = Math.floor(diffSec / 3600)
    return locale.startsWith('zh') ? `${h} 小时前` : `${h}h ago`
  }
  return date.toLocaleString(locale)
}

export function formatLocalTime(isoString: string, locale: string): string {
  return new Date(isoString).toLocaleTimeString(locale, {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

export function formatLocalDateTime(isoString: string, locale: string): string {
  return new Date(isoString).toLocaleString(locale, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

export function formatCount(n: number | null | undefined, locale: string): string {
  if (n == null) return '—'
  return n.toLocaleString(locale)
}

export function formatNumber(n: number, decimals = 1): string {
  return n.toFixed(decimals)
}

export function formatPercent(n: number): string {
  return `${n.toFixed(1)}%`
}

export function formatDelay(ms: number, locale: string): string {
  return locale.startsWith('zh') ? `${ms} 毫秒` : `${ms}ms`
}
