/**
 * Adapted from shadcn/ui, Copyright (c) 2023 shadcn (MIT).
 * See THIRD_PARTY_LICENSES.txt in the source distribution.
 */
import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
