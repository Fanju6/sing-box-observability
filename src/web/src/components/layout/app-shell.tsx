/**
 * Layout structure adapted in part from sing-box-dashboard.
 * Copyright (C) 2022 nekohasekai <contact-sagernet@sekai.icu>.
 * Modifications Copyright (C) 2026 Fanju and contributors.
 * See NOTICE and THIRD_PARTY_LICENSES.txt.
 */
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Sidebar } from './sidebar'
import { Header } from './header'
import { TooltipProvider } from '@/components/ui/tooltip'

export function AppShell({ children }: { children: React.ReactNode }) {
  const { t } = useTranslation()
  const [sidebarOpen, setSidebarOpen] = useState(false)

  return (
    <TooltipProvider>
      <div className="flex h-full min-w-0 overflow-hidden bg-[var(--color-canvas)] text-[var(--color-text)]">
        {sidebarOpen && (
          <button
            type="button"
            className="fixed inset-0 z-40 border-0 bg-black/30 lg:hidden"
            aria-label={t('nav.close')}
            onClick={() => setSidebarOpen(false)}
          />
        )}
        <Sidebar open={sidebarOpen} onNavigate={() => setSidebarOpen(false)} />
        <div className="flex min-w-0 flex-1 flex-col">
          <Header open={sidebarOpen} onToggle={() => setSidebarOpen((value) => !value)} />
          <main className="min-h-0 flex-1 overflow-y-auto overscroll-none">
            <div className="app-page">{children}</div>
          </main>
        </div>
      </div>
    </TooltipProvider>
  )
}
