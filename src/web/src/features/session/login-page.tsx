import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useLogin } from '@/api/hooks'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useTheme, type Theme } from '@/app/theme-context'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useNavigate } from 'react-router-dom'

export function LoginPage() {
  const { t } = useTranslation()
  const { theme, setTheme } = useTheme()
  const [password, setPassword] = useState('')
  const login = useLogin()
  const navigate = useNavigate()
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    login.mutate(password, {
      onSuccess: () => navigate('/overview', { replace: true }),
      onError: (err: Error) => setError(err.message),
    })
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-[var(--color-canvas)] p-4">
      <div className="absolute right-4 top-[calc(16px+env(safe-area-inset-top))]">
        <Select value={theme} onValueChange={(v) => setTheme(v as Theme)}>
          <SelectTrigger className="w-28 text-xs">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="light">{t('settings.themeLight')}</SelectItem>
            <SelectItem value="dark">{t('settings.themeDark')}</SelectItem>
            <SelectItem value="system">{t('settings.themeSystem')}</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div className="w-full max-w-sm">
        <div className="mb-7 text-center">
          <h1 className="font-serif text-[27px] font-semibold tracking-[-0.025em]">sing-box-observability</h1>
          <p className="mt-4 text-[13px] text-[var(--color-text-muted)]">{t('login.subtitle')}</p>
        </div>

        <form onSubmit={handleSubmit} className="surface-card space-y-4 p-5">
          <div>
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder={t('settings.tokenPlaceholder')}
              autoComplete="current-password"
              disabled={login.isPending}
              autoFocus
            />
          </div>
          {error && <p className="text-sm text-[var(--color-danger)]" role="alert">{error}</p>}
          <Button type="submit" className="w-full" disabled={login.isPending || !password}>
            {login.isPending ? t('common.loading') : t('login.submit')}
          </Button>
        </form>
      </div>
    </div>
  )
}
