/**
 * Layout structure adapted in part from sing-box-dashboard.
 * Copyright (C) 2022 nekohasekai <contact-sagernet@sekai.icu>.
 * Modifications Copyright (C) 2026 Fanju and contributors.
 * See NOTICE and THIRD_PARTY_LICENSES.txt.
 */
import { Menu, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'

export function Header({ open, onToggle }: { open: boolean; onToggle: () => void }) {
  const { t } = useTranslation()
  const Icon = open ? X : Menu

  return (
    <header className="relative z-[45] flex shrink-0 items-center gap-2 border-b border-[var(--color-border)] bg-[var(--color-surface-2)] px-2.5 pb-1.5 pt-[calc(6px+env(safe-area-inset-top))] lg:hidden">
      <button
        type="button"
        aria-label={t('nav.toggle')}
        aria-expanded={open}
        onClick={onToggle}
        className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[8px] border-0 bg-transparent text-[var(--color-text-muted)] hover:bg-[var(--color-hover)] hover:text-[var(--color-text)]"
      >
        <Icon className="h-[18px] w-[18px]" strokeWidth={1.8} />
      </button>
      <div className="truncate font-serif text-[17px] font-semibold tracking-[-0.015em]">sing-box-observability</div>
    </header>
  )
}
