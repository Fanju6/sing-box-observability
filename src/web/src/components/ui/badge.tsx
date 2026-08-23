/**
 * Adapted from shadcn/ui, Copyright (c) 2023 shadcn (MIT).
 * See THIRD_PARTY_LICENSES.txt in the source distribution.
 */
import * as React from 'react'
import { cn } from '@/lib/cn'
import { cva, type VariantProps } from 'class-variance-authority'

const badgeVariants = cva(
  'inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold tabular-nums transition-colors',
  {
    variants: {
      variant: {
        default: 'bg-[var(--color-primary-soft)] text-[var(--color-text-muted)]',
        healthy: 'bg-[var(--color-healthy-soft)] text-[var(--color-healthy)]',
        warning: 'bg-[var(--color-warning-soft)] text-[var(--color-warning)]',
        danger: 'bg-[var(--color-danger-soft)] text-[var(--color-danger)]',
        upload: 'bg-[var(--color-upload-soft)] text-[var(--color-upload)]',
        download: 'bg-[var(--color-download-soft)] text-[var(--color-download)]',
        primary: 'bg-[var(--color-primary-soft)] text-[var(--color-primary)]',
        outline: 'border border-[var(--color-border)] text-[var(--color-text-muted)]',
      },
    },
    defaultVariants: { variant: 'default' },
  },
)

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement>, VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, ...props }: BadgeProps) {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />
}

export { Badge }
