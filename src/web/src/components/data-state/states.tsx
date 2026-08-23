import { useTranslation } from 'react-i18next'
import { AlertCircle, Inbox, RefreshCw, Lock } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface EmptyStateProps {
  icon?: React.ReactNode
  title?: string
  description?: string
  action?: { label: string; onClick: () => void }
  className?: string
}

export function EmptyState({ icon, title, description, action, className }: EmptyStateProps) {
  const { t } = useTranslation()
  return (
    <div className={`flex flex-col items-center justify-center px-5 py-16 text-center text-[13px] text-[var(--color-text-faint)] ${className ?? ''}`}>
      <div className="mb-2.5 text-[var(--color-border-strong)]">
        {icon || <Inbox className="h-7 w-7" />}
      </div>
      <div>{title || t('common.empty')}</div>
      {description && <p className="mt-1 max-w-sm text-xs text-[var(--color-text-muted)]">{description}</p>}
      {action && (
        <Button className="mt-4" variant="secondary" size="sm" onClick={action.onClick}>
          {action.label}
        </Button>
      )}
    </div>
  )
}

interface ErrorStateProps {
  error?: Error | { code?: string; message?: string } | null
  onRetry?: () => void
}

export function ErrorState({ error, onRetry }: ErrorStateProps) {
  const { t } = useTranslation()
  const isUnauthorized = error && 'code' in error && error.code === 'UPSTREAM_UNAUTHORIZED'
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <div className="mb-4 rounded-full bg-[color-mix(in_srgb,var(--color-danger)_15%,transparent)] p-4 text-[var(--color-danger)]">
        {isUnauthorized ? <Lock className="h-8 w-8" /> : <AlertCircle className="h-8 w-8" />}
      </div>
      <h3 className="mb-1 text-sm font-semibold text-[var(--color-text)]">
        {isUnauthorized ? t('source.unauthorized') : t('common.error')}
      </h3>
      {error?.message && (
        <p className="mb-4 max-w-sm text-xs text-[var(--color-text-muted)]">{error.message}</p>
      )}
      {onRetry && !isUnauthorized && (
        <Button variant="secondary" size="sm" onClick={onRetry}>
          <RefreshCw className="mr-1 h-3 w-3" />
          {t('common.retry')}
        </Button>
      )}
    </div>
  )
}

interface SkeletonProps {
  className?: string
}

export function CardSkeleton({ className }: SkeletonProps) {
  return (
    <div className={`surface-card p-3.5 lg:p-5 ${className ?? ''}`}>
      <div className="mb-3 h-3 w-20 animate-pulse rounded bg-[var(--color-surface-2)]" />
      <div className="mb-1 h-7 w-32 animate-pulse rounded bg-[var(--color-surface-2)]" />
      <div className="h-3 w-24 animate-pulse rounded bg-[var(--color-surface-2)]" />
    </div>
  )
}

export function ChartSkeleton({ className }: SkeletonProps) {
  return (
    <div className={`surface-card p-3.5 lg:p-5 ${className ?? ''}`}>
      <div className="mb-4 h-4 w-32 animate-pulse rounded bg-[var(--color-surface-2)]" />
      <div className="h-48 animate-pulse rounded bg-[var(--color-surface-2)]" />
    </div>
  )
}
