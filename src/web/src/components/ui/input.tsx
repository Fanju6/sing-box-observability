/**
 * Adapted from shadcn/ui, Copyright (c) 2023 shadcn (MIT).
 * See THIRD_PARTY_LICENSES.txt in the source distribution.
 */
import * as React from 'react'
import { cn } from '@/lib/cn'

const Input = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(
  ({ className, type, ...props }, ref) => {
    return (
      <input
        type={type}
        className={cn(
          'flex h-[var(--control-height)] w-full rounded-[10px] border border-[var(--color-border)] bg-[var(--color-surface-1)] px-3 text-[13px] shadow-[var(--shadow-control)] transition-[border-color,box-shadow] file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-[var(--color-text-faint)] focus-visible:border-[var(--color-primary)] focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-[var(--color-primary-soft)] disabled:cursor-not-allowed disabled:opacity-50 max-lg:text-[16px]',
          className,
        )}
        ref={ref}
        {...props}
      />
    )
  },
)
Input.displayName = 'Input'

export { Input }
