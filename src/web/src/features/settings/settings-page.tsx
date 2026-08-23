import { useTranslation } from 'react-i18next'
import i18n from '@/i18n'
import { useTheme } from '@/app/theme-context'
import { useMeta, useSession, useLogout } from '@/api/hooks'
import type { Theme } from '@/app/theme-context'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { StatusDot } from '@/components/data-state/status-dot'
import { formatDuration, formatLocalDateTime } from '@/lib/format'
import { CardSkeleton } from '@/components/data-state/states'
import { PageHeader } from '@/components/layout/page-header'
import { LogOut } from 'lucide-react'

const themes: { value: Theme; labelKey: string }[] = [
  { value: 'light', labelKey: 'settings.themeLight' },
  { value: 'dark', labelKey: 'settings.themeDark' },
  { value: 'system', labelKey: 'settings.themeSystem' },
]

const languages = [
  { value: 'zh-CN', labelKey: 'settings.languageZh' },
  { value: 'en', labelKey: 'settings.languageEn' },
]

function SettingsSection({ title, children }: {
  title: string
  children: React.ReactNode
}) {
  return (
    <section>
      <h2 className="px-1 pb-2 text-[14px] font-normal text-[var(--color-text-faint)]">{title}</h2>
      <div className="surface-card divide-y divide-[var(--color-border)]">{children}</div>
    </section>
  )
}

function SettingRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 px-4 py-3.5">
      <span className="text-[13px] text-[var(--color-text)]">{label}</span>
      <span className="min-w-0 text-right text-[13px]">{value}</span>
    </div>
  )
}

function stateVariant(state: string) {
  if (state === 'online') return 'healthy' as const
  if (state === 'connecting' || state === 'stale') return 'warning' as const
  return 'danger' as const
}

export function SettingsPage() {
  const { t } = useTranslation()
  const { theme, setTheme } = useTheme()
  const { data: meta, isLoading: metaLoading } = useMeta()
  const { data: session } = useSession()
  const logout = useLogout()
  const locale = i18n.language

  return (
    <div className="min-w-0">
      <PageHeader title={t('settings.title')} />

      <div className="flex flex-col gap-4">

      <SettingsSection title={t('settings.appearance')}>
        <SettingRow
          label={t('settings.theme')}
          value={
            <Select value={theme} onValueChange={(v) => setTheme(v as Theme)}>
              <SelectTrigger className="w-[150px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {themes.map((th) => (
                  <SelectItem key={th.value} value={th.value}>{t(th.labelKey)}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          }
        />
        <SettingRow
          label={t('settings.language')}
          value={
            <Select value={i18n.language} onValueChange={(v) => i18n.changeLanguage(v)}>
              <SelectTrigger className="w-[150px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {languages.map((l) => (
                  <SelectItem key={l.value} value={l.value}>{t(l.labelKey)}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          }
        />
      </SettingsSection>

      {metaLoading ? (
        <CardSkeleton />
      ) : meta ? (
        <SettingsSection title={t('settings.dataSource')}>
          <SettingRow label={t('settings.displayName')} value={<span className="font-mono">{meta.source.displayName}</span>} />
          <SettingRow
            label={t('settings.state')}
            value={
              <span className="flex items-center gap-2">
                <StatusDot state={meta.source.state} size="sm" />
                {t(`common.${meta.source.state}`)}
              </span>
            }
          />
          {meta.source.lastAttemptAt && (
            <SettingRow
              label={t('source.lastAttempt')}
              value={<span className="font-mono text-xs">{formatLocalDateTime(meta.source.lastAttemptAt, locale)}</span>}
            />
          )}
          {meta.source.lastSuccessAt && (
            <SettingRow
              label={t('common.lastSuccess')}
              value={<span className="font-mono text-xs">{formatLocalDateTime(meta.source.lastSuccessAt, locale)}</span>}
            />
          )}
          {meta.source.lastErrorCode && (
            <SettingRow label={t('source.errorCode')} value={<Badge variant="danger">{meta.source.lastErrorCode}</Badge>} />
          )}
          {meta.source.historyAvailableFrom && (
            <SettingRow
              label={t('settings.historyStart')}
              value={<span className="font-mono text-xs">{formatLocalDateTime(meta.source.historyAvailableFrom, locale)}</span>}
            />
          )}
        </SettingsSection>
      ) : null}

      {meta && (
        <>
          <SettingsSection title={t('settings.collector')}>
            <SettingRow
              label={t('settings.scrapeInterval')}
              value={<span className="font-mono text-xs">{meta.collector.scrapeIntervalSeconds}s</span>}
            />
            <SettingRow
              label={t('settings.persistInterval')}
              value={<span className="font-mono text-xs">{meta.collector.persistIntervalSeconds}s</span>}
            />
            <SettingRow
              label={t('settings.retention')}
              value={<span className="font-mono text-xs">{formatDuration(meta.collector.retentionSeconds)}</span>}
            />
            <SettingRow
              label={t('settings.maxPoints')}
              value={<span className="font-mono text-xs">{meta.collector.maxSeriesPoints}</span>}
            />
            {Object.entries(meta.collector.channels).map(([channel, health]) => (
              <SettingRow
                key={channel}
                label={t(`overview.channel.${channel}`)}
                value={<Badge variant={stateVariant(health.state)}>{t(`common.${health.state}`)}</Badge>}
              />
            ))}
          </SettingsSection>

          {meta.capabilities && (
            <SettingsSection title={t('settings.capabilities')}>
            <SettingRow
              label={t('settings.upstreamApiVersion')}
              value={<Badge variant="primary">v{meta.capabilities.upstreamApiVersion}</Badge>}
            />
            <SettingRow
              label={t('settings.cursorPagination')}
              value={
                meta.capabilities.cursorPagination
                  ? <Badge variant="healthy">{t('settings.enabled')}</Badge>
                  : <Badge variant="danger">{t('settings.disabled')}</Badge>
              }
            />
            <SettingRow
              label={t('settings.eventReplay')}
              value={
                meta.capabilities.eventReplay
                  ? <Badge variant="healthy">{t('settings.enabled')}</Badge>
                  : <Badge variant="outline">{t('settings.disabled')}</Badge>
              }
            />
            <SettingRow
              label={t('settings.exposeSensitive')}
              value={
                meta.capabilities.exposeSensitive
                  ? <Badge variant="healthy">{t('settings.enabled')}</Badge>
                  : <Badge variant="outline">{t('settings.disabled')}</Badge>
              }
            />
            <SettingRow
              label={t('settings.recentLimit')}
              value={<span className="font-mono text-xs">{meta.capabilities.recentConnectionLimit}</span>}
            />
            <SettingRow
              label={t('settings.recentTtl')}
              value={<span className="font-mono text-xs">{formatDuration(meta.capabilities.recentTtlSeconds)}</span>}
            />
            <SettingRow
              label={t('settings.topKSize')}
              value={<span className="font-mono text-xs">{meta.capabilities.topKSize}</span>}
            />
            <SettingRow
              label={t('settings.activePageLimit')}
              value={<span className="font-mono text-xs">{meta.capabilities.activePageLimit}</span>}
            />
            <SettingRow
              label={t('rankings.dimension')}
              value={
                <div className="flex flex-wrap gap-1 justify-end max-w-[200px]">
                  {meta.capabilities.rankingDimensions.map((d) => (
                    <Badge key={d} variant="outline" className="text-[10px]">{d}</Badge>
                  ))}
                </div>
              }
            />
            <SettingRow
              label={t('settings.endpoints')}
              value={
                <div className="flex flex-wrap gap-1 justify-end max-w-[260px]">
                  {meta.capabilities.endpoints.map((endpoint) => (
                    <Badge key={endpoint} variant="outline" className="text-[10px] font-mono">{endpoint}</Badge>
                  ))}
                </div>
              }
            />
            </SettingsSection>
          )}
        </>
      )}

      {session?.authEnabled && (
        <SettingsSection title={t('settings.auth')}>
          <SettingRow
            label={t('settings.auth')}
            value={
              session.authenticated
                ? <Badge variant="healthy">{t('common.online')}</Badge>
                : <Badge variant="danger">{t('settings.login')}</Badge>
            }
          />
          {session.authenticated && (
            <div className="pt-2">
              <Button variant="secondary" size="sm" onClick={() => logout.mutate()}>
                <LogOut className="mr-2 h-3 w-3" />
                {t('settings.logout')}
              </Button>
            </div>
          )}
        </SettingsSection>
      )}

      {meta && (
        <div className="text-center text-[10px] text-[var(--color-text-faint)] py-4">
          sing-box-observability · v{meta.appVersion}
        </div>
      )}
      </div>
    </div>
  )
}
