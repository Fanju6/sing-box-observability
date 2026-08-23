/**
 * Layout structure adapted in part from sing-box-dashboard.
 * Copyright (C) 2022 nekohasekai <contact-sagernet@sekai.icu>.
 * Modifications Copyright (C) 2026 Fanju and contributors.
 * See NOTICE and THIRD_PARTY_LICENSES.txt.
 */
import { NavLink, useLocation } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { BarChart3, ChartNoAxesCombined, LayoutDashboard, Network, Settings } from 'lucide-react'
import { cn } from '@/lib/cn'
import { StatusDot } from '@/components/data-state/status-dot'
import { useMeta } from '@/api/hooks'

const navItems = [
  { to: '/overview', icon: LayoutDashboard, key: 'nav.overview' },
  { to: '/trends', icon: ChartNoAxesCombined, key: 'nav.trends' },
  { to: '/connections', icon: Network, key: 'nav.connections' },
  { to: '/rankings', icon: BarChart3, key: 'nav.rankings' },
  { to: '/settings', icon: Settings, key: 'nav.settings' },
] as const

export function Sidebar({ open, onNavigate }: { open: boolean; onNavigate: () => void }) {
  const { t } = useTranslation()
  const { data: meta } = useMeta()
  const location = useLocation()

  return (
    <aside
      data-open={open ? 'true' : 'false'}
      className={cn(
        'fixed inset-y-0 left-0 z-50 flex h-full w-[min(78vw,300px)] shrink-0 -translate-x-full flex-col border-r border-[var(--color-border)] bg-[var(--color-surface-2)] px-3 pb-[calc(14px+env(safe-area-inset-bottom))] pt-[calc(20px+env(safe-area-inset-top))] shadow-[var(--shadow-overlay)] transition-transform duration-200 lg:static lg:z-auto lg:w-[232px] lg:translate-x-0 lg:pb-3 lg:pt-5 lg:shadow-none',
        open && 'translate-x-0',
      )}
    >
      <div className="px-2 pb-3.5">
        <div className="font-serif text-[17px] font-semibold leading-none tracking-[-0.02em]">sing-box-observability</div>
      </div>

      <nav className="flex flex-1 flex-col gap-px" aria-label={t('nav.main')}>
        {navItems.map((item) => {
          const Icon = item.icon
          const active = location.pathname.startsWith(item.to)
          return (
            <NavLink
              key={item.to}
              to={item.to}
              onClick={onNavigate}
              className={cn(
                'flex items-center gap-2.5 rounded-[10px] px-2.5 py-2.5 text-sm font-medium transition-[background,color] duration-150 lg:py-2 lg:text-[13px]',
                active
                  ? 'bg-[var(--color-primary-soft)] font-semibold text-[var(--color-text)]'
                  : 'text-[var(--color-text-muted)] hover:bg-[var(--color-hover)] hover:text-[var(--color-text)]',
              )}
            >
              <Icon className={cn('h-[18px] w-[18px]', active ? 'text-[var(--color-primary)]' : 'text-[var(--color-text-faint)]')} strokeWidth={1.8} />
              {t(item.key)}
            </NavLink>
          )
        })}
      </nav>

      {meta && (
        <div className="border-t border-[var(--color-border)] pt-3">
          <div className="flex items-center gap-2 rounded-[10px] px-2.5 py-2">
            <StatusDot state={meta.source.state} size="sm" />
            <div className="min-w-0 flex-1">
              <div className="truncate text-[13px] font-medium">{meta.source.displayName}</div>
              <div className="font-mono text-[10px] text-[var(--color-text-faint)]">v{meta.appVersion}</div>
            </div>
          </div>
        </div>
      )}
    </aside>
  )
}
