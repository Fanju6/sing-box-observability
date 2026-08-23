/**
 * Component structure adapted in part from sing-box-dashboard.
 * Copyright (C) 2022 nekohasekai <contact-sagernet@sekai.icu>.
 * Modifications Copyright (C) 2026 Fanju and contributors.
 * See NOTICE and THIRD_PARTY_LICENSES.txt.
 */
import { cn } from '@/lib/cn'
import { ArrowLeft } from 'lucide-react'

export function PageHeader({
  title,
  actions,
  back,
  className,
}: {
  title: string
  actions?: React.ReactNode
  back?: { label: string; onClick: () => void }
  className?: string
}) {
  return (
    <div className={cn('mb-6 flex shrink-0 items-center gap-3 max-lg:mb-4 max-lg:flex-wrap max-lg:gap-y-2.5', className)}>
      {back && (
        <button type="button" aria-label={back.label} onClick={back.onClick} className="flex shrink-0 items-center justify-center rounded-[8px] border-0 bg-transparent p-1.5 text-[var(--color-text-muted)] hover:bg-[var(--color-hover)] hover:text-[var(--color-text)]">
          <ArrowLeft className="h-5 w-5" strokeWidth={1.8} />
        </button>
      )}
      <h1 className="page-title">{title}</h1>
      {actions && <div className="ml-auto flex items-center gap-2 max-lg:flex-wrap max-lg:justify-end">{actions}</div>}
    </div>
  )
}
