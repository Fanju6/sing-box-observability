/**
 * Adapted from shadcn/ui, Copyright (c) 2023 shadcn (MIT).
 * See THIRD_PARTY_LICENSES.txt in the source distribution.
 */
import { cn } from '@/lib/cn'

function Skeleton({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn('animate-pulse rounded-md bg-[var(--color-surface-2)]', className)}
      {...props}
    />
  )
}

export { Skeleton }
