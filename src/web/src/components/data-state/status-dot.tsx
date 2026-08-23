import { cn } from '@/lib/cn'
import type { SourceState } from '@/api/types'
import { Wifi, WifiOff, AlertTriangle, Lock, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

interface StatusDotProps {
  state: SourceState
  size?: 'sm' | 'md' | 'lg'
  showLabel?: boolean
  className?: string
}

const stateConfig: Record<SourceState, { color: string; icon: React.ElementType; labelKey: string }> = {
  online: { color: 'bg-[var(--color-healthy)]', icon: Wifi, labelKey: 'common.online' },
  stale: { color: 'bg-[var(--color-warning)]', icon: AlertTriangle, labelKey: 'common.stale' },
  offline: { color: 'bg-[var(--color-danger)]', icon: WifiOff, labelKey: 'common.offline' },
  unauthorized: { color: 'bg-[var(--color-danger)]', icon: Lock, labelKey: 'common.unauthorized' },
  connecting: { color: 'bg-[var(--color-text-faint)] animate-[soft-pulse_1.4s_ease-in-out_infinite]', icon: Loader2, labelKey: 'common.connecting' },
}

export function StatusDot({ state, size = 'md', showLabel = false, className }: StatusDotProps) {
  const { t } = useTranslation()
  const config = stateConfig[state]
  const Icon = config.icon
  const sizeClass = size === 'sm' ? 'h-2 w-2' : size === 'lg' ? 'h-3 w-3' : 'h-2.5 w-2.5'

  return (
    <span className={cn('inline-flex items-center gap-1.5', className)}>
      <span className={cn('relative flex h-2 w-2', size === 'sm' ? 'h-2 w-2' : size === 'lg' ? 'h-3 w-3' : 'h-2.5 w-2.5')}>
        <span className={cn('relative inline-flex rounded-full', sizeClass, config.color)} />
      </span>
      {showLabel && (
        <span className="text-xs font-medium text-[var(--color-text-muted)]">
          <Icon className="mr-1 inline h-3 w-3" />
          {t(config.labelKey)}
        </span>
      )}
    </span>
  )
}
