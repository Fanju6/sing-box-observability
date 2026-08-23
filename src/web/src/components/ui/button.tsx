/**
 * Adapted from shadcn/ui, Copyright (c) 2023 shadcn (MIT).
 * See THIRD_PARTY_LICENSES.txt in the source distribution.
 */
import * as React from 'react'
import { cn } from '@/lib/cn'
import { cva, type VariantProps } from 'class-variance-authority'

const buttonVariants = cva(
  'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-[10px] border font-medium shadow-[var(--shadow-control)] transition-[background,border-color,color,opacity] duration-150 focus-visible:outline-none disabled:pointer-events-none disabled:opacity-50',
  {
    variants: {
      variant: {
        default: 'border-transparent bg-[var(--color-primary)] text-[var(--color-primary-fg)] hover:opacity-88',
        secondary: 'border-[var(--color-border)] bg-[var(--color-surface-1)] text-[var(--color-text)] hover:bg-[var(--color-inset)]',
        ghost: 'border-transparent bg-transparent shadow-none text-[var(--color-text-muted)] hover:bg-[var(--color-hover)] hover:text-[var(--color-text)]',
        outline: 'border-[var(--color-border)] bg-transparent text-[var(--color-text)] hover:bg-[var(--color-hover)]',
        destructive: 'border-transparent bg-[var(--color-danger)] text-white hover:opacity-90',
      },
      size: {
        default: 'h-[var(--control-height)] px-3.5 text-[13px]',
        sm: 'h-6 px-2.5 text-xs max-lg:h-7',
        lg: 'h-10 px-5 text-sm',
        icon: 'h-[var(--control-height)] w-[var(--control-height)] rounded-[8px] p-0',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
)

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, ...props }, ref) => {
    return (
      <button className={cn(buttonVariants({ variant, size, className }))} ref={ref} {...props} />
    )
  },
)
Button.displayName = 'Button'

export { Button }
