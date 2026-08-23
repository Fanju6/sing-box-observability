import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AppProviders } from './providers'
import { SSEProvider } from './sse-provider'
import { AppShell } from '../components/layout/app-shell'
import { useSession } from '../api/hooks'
import { lazy, Suspense, type ReactNode } from 'react'
import { useEffect } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { queryKeys } from '../api/query-keys'
import type { SessionResponse } from '../api/types'
import { useTranslation } from 'react-i18next'

const LoginPage = lazy(() => import('../features/session/login-page').then((module) => ({ default: module.LoginPage })))
const OverviewPage = lazy(() => import('../features/overview/overview-page').then((module) => ({ default: module.OverviewPage })))
const TrendsPage = lazy(() => import('../features/trends/trends-page').then((module) => ({ default: module.TrendsPage })))
const ConnectionsPage = lazy(() => import('../features/connections/connections-page').then((module) => ({ default: module.ConnectionsPage })))
const RankingsPage = lazy(() => import('../features/rankings/rankings-page').then((module) => ({ default: module.RankingsPage })))
const SettingsPage = lazy(() => import('../features/settings/settings-page').then((module) => ({ default: module.SettingsPage })))

function RouteFallback() {
  const { t } = useTranslation()
  return (
    <div className="min-h-screen flex items-center justify-center bg-[var(--color-bg)]">
      <div className="animate-pulse text-sm text-[var(--color-text-muted)]">{t('common.loading')}</div>
    </div>
  )
}

function RequireAuth({ children }: { children: ReactNode }) {
  const { t } = useTranslation()
  const { data: session, isLoading, isError, refetch } = useSession()
  const queryClient = useQueryClient()
  useEffect(() => {
    const handleSessionExpired = () => {
      queryClient.setQueryData(queryKeys.session, { authEnabled: true, authenticated: false } satisfies SessionResponse)
    }
    window.addEventListener('observability:session-expired', handleSessionExpired)
    return () => window.removeEventListener('observability:session-expired', handleSessionExpired)
  }, [queryClient])
  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[var(--color-bg)]">
        <div className="animate-pulse text-sm text-[var(--color-text-muted)]">{t('common.loading')}</div>
      </div>
    )
  }
  if (isError) {
    return (
      <div className="min-h-screen flex flex-col gap-3 items-center justify-center bg-[var(--color-bg)] text-sm text-[var(--color-text-muted)]">
        <span>{t('common.sessionError')}</span>
        <button className="text-[var(--color-primary)] underline" onClick={() => refetch()}>{t('common.retry')}</button>
      </div>
    )
  }
  if (session?.authEnabled && !session.authenticated) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}

function ProtectedRoutes() {
  return (
    <RequireAuth>
      <SSEProvider>
        <AppShell>
          <Suspense fallback={<RouteFallback />}>
            <Routes>
              <Route path="/" element={<Navigate to="/overview" replace />} />
              <Route path="/overview" element={<OverviewPage />} />
              <Route path="/trends" element={<TrendsPage />} />
              <Route path="/connections" element={<ConnectionsPage />} />
              <Route path="/rankings" element={<RankingsPage />} />
              <Route path="/settings" element={<SettingsPage />} />
              <Route path="*" element={<Navigate to="/overview" replace />} />
            </Routes>
          </Suspense>
        </AppShell>
      </SSEProvider>
    </RequireAuth>
  )
}

export default function App() {
  return (
    <AppProviders>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Suspense fallback={<RouteFallback />}><LoginPage /></Suspense>} />
          <Route path="/*" element={<ProtectedRoutes />} />
        </Routes>
      </BrowserRouter>
    </AppProviders>
  )
}
