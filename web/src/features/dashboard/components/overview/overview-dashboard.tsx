import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Copy, KeyRound, Link2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { fetchTokenKey, getApiKeys } from '@/features/keys/api'
import type { ApiKey } from '@/features/keys/types'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { ROLE } from '@/lib/roles'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import { useDashboardStatus } from '../../hooks/use-status-data'
import { resolvePrimaryApiAddress } from '../../lib/api-info'
import { AnnouncementsPanel } from './announcements-panel'
import { FAQPanel } from './faq-panel'
import { SummaryCards } from './summary-cards'
import { UptimePanel } from './uptime-panel'
import { UsageTrendPanel } from './usage-trend-panel'

type DashboardActionPath =
  | '/keys'
  | '/wallet'
  | '/playground'
  | '/channels'
  | '/usage-logs'
  | '/pricing'

interface QuickAction {
  title: string
  to: DashboardActionPath
}

function getCurrentOrigin(): string {
  if (typeof window === 'undefined') return ''
  return window.location.origin
}

function getPreferredKey(keys: ApiKey[]): ApiKey | null {
  return keys.find((item) => item.status === 1) ?? keys[0] ?? null
}

function normalizeApiKey(key: string): string {
  return key.startsWith('sk-') ? key : `sk-${key}`
}

function formatMaskedKey(key: string): string {
  const fullKey = normalizeApiKey(key)
  if (fullKey.length <= 16) return fullKey
  return `${fullKey.slice(0, 7)}...${fullKey.slice(-4)}`
}

function ConnectionOverview(props: {
  apiAddress: string
  keyItem: ApiKey | null
  keysLoading: boolean
  actions: QuickAction[]
}) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard({ notify: false })
  const [keyCopying, setKeyCopying] = useState(false)

  const handleCopyAddress = async () => {
    const copied = await copyToClipboard(props.apiAddress)
    if (copied) {
      toast.success(t('Copied to clipboard'))
    } else {
      toast.error(t('Failed to copy to clipboard'))
    }
  }

  const handleCopyKey = async () => {
    if (!props.keyItem || keyCopying) return

    setKeyCopying(true)
    try {
      const result = await fetchTokenKey(props.keyItem.id)
      if (!result.success || !result.data?.key) {
        toast.error(result.message || t('Failed to copy to clipboard'))
        return
      }

      const copied = await copyToClipboard(normalizeApiKey(result.data.key))
      if (copied) {
        toast.success(t('Copied to clipboard'))
        return
      }
      toast.error(t('Failed to copy to clipboard'))
    } catch {
      toast.error(t('Failed to copy to clipboard'))
    } finally {
      setKeyCopying(false)
    }
  }

  const keyIsReady = props.keyItem?.status === 1
  const statusLabel = props.keyItem
    ? t(keyIsReady ? 'Ready' : 'Disabled')
    : t('Not configured')

  return (
    <section
      aria-label={t('API Connection')}
      className='bg-card overflow-hidden rounded-2xl border'
    >
      <div className='p-4 sm:p-5'>
        <h3 className='flex items-center gap-2 text-sm font-semibold'>
          <Link2 aria-hidden='true' className='size-4' />
          {t('API Connection')}
        </h3>

        <div
          data-testid='overview-connection-grid'
          className='mt-4 grid min-w-0 gap-4 lg:grid-cols-[minmax(0,1.15fr)_minmax(0,0.9fr)_minmax(9rem,0.45fr)] xl:grid-cols-[minmax(0,1.2fr)_minmax(0,0.9fr)_minmax(9rem,0.45fr)_auto]'
        >
          <div className='min-w-0'>
            <p className='text-muted-foreground mb-2 text-xs font-medium'>
              {t('API address')}
            </p>
            <div className='border-input bg-muted/30 flex h-9 min-w-0 items-center gap-2 rounded-md border px-3'>
              <code
                className='min-w-0 flex-1 truncate text-xs font-medium'
                title={props.apiAddress}
              >
                {props.apiAddress}
              </code>
              <Button
                variant='ghost'
                size='icon-sm'
                className='-mr-2 size-7 shrink-0'
                onClick={handleCopyAddress}
                aria-label={t('Copy API address')}
              >
                <Copy />
              </Button>
            </div>
          </div>

          <div className='min-w-0'>
            <p className='text-muted-foreground mb-2 text-xs font-medium'>
              {t('API key')}
            </p>
            {props.keysLoading && <Skeleton className='h-9 w-full' />}
            {!props.keysLoading && props.keyItem && (
              <div className='border-input bg-muted/30 flex h-9 min-w-0 items-center gap-2 rounded-md border px-3'>
                <code
                  className='min-w-0 flex-1 truncate text-xs font-medium'
                  title={props.keyItem.name}
                >
                  {formatMaskedKey(props.keyItem.key)}
                </code>
                <Button
                  variant='ghost'
                  size='icon-sm'
                  className='-mr-2 size-7 shrink-0'
                  onClick={handleCopyKey}
                  disabled={keyCopying}
                  aria-label={t('Copy API key')}
                >
                  <Copy />
                </Button>
              </div>
            )}
            {!props.keysLoading && !props.keyItem && (
              <div className='border-input bg-muted/30 text-muted-foreground flex h-9 items-center rounded-md border px-3 text-xs'>
                {t('No API key yet')}
              </div>
            )}
          </div>

          <div className='min-w-0'>
            <p className='text-muted-foreground mb-2 text-xs font-medium'>
              {t('Status')}
            </p>
            <div className='flex h-9 items-center gap-2 text-sm font-medium'>
              <span
                aria-hidden='true'
                className={cn(
                  'size-2 rounded-full',
                  keyIsReady ? 'bg-success' : 'bg-muted-foreground'
                )}
              />
              <span className='truncate'>{statusLabel}</span>
            </div>
          </div>

          <div className='flex flex-wrap items-end gap-2 lg:col-span-3 xl:col-span-1 xl:flex-nowrap'>
            <Button variant='outline' size='sm' onClick={handleCopyAddress}>
              <Copy />
              {t('Copy API address')}
            </Button>
            <Button variant='outline' size='sm' render={<Link to='/keys' />}>
              <KeyRound />
              {t('Manage API Keys')}
            </Button>
          </div>
        </div>
      </div>

      <nav aria-label={t('Quick actions')} className='border-t'>
        <div className='flex flex-wrap items-center gap-y-2 px-4 py-3 sm:px-5'>
          {props.actions.map((action, index) => (
            <div key={action.title} className='flex items-center'>
              {index > 0 && (
                <span
                  aria-hidden='true'
                  className='bg-border mx-4 h-4 w-px sm:mx-5'
                />
              )}
              <Button
                variant='link'
                size='sm'
                className='text-foreground h-auto p-0 text-xs font-medium no-underline hover:no-underline'
                render={<Link to={action.to} />}
              >
                {action.title}
              </Button>
            </div>
          ))}
        </div>
      </nav>
    </section>
  )
}

export function OverviewDashboard() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const {
    serverAddress,
    announcements: showAnnouncementsPanel,
    faq: showFAQPanel,
    uptimeKuma: showUptimePanel,
  } = useDashboardStatus()
  const apiAddress = resolvePrimaryApiAddress(serverAddress, getCurrentOrigin())

  const isAdmin = Boolean(user?.role && user.role >= ROLE.ADMIN)

  const apiKeysQuery = useQuery({
    queryKey: ['dashboard', 'overview', 'api-keys'],
    queryFn: async () => {
      const result = await getApiKeys({ p: 1, size: 10 })
      return result.success ? (result.data?.items ?? []) : []
    },
    staleTime: 60 * 1000,
  })

  const preferredKey = useMemo(
    () => getPreferredKey(apiKeysQuery.data ?? []),
    [apiKeysQuery.data]
  )

  const quickActions = useMemo<QuickAction[]>(
    () => [
      {
        title: t('API Keys'),
        to: '/keys',
      },
      {
        title: t('Usage Logs'),
        to: '/usage-logs',
      },
      {
        title: t('Pricing'),
        to: '/pricing',
      },
      {
        title: isAdmin ? t('Channels') : t('Wallet'),
        to: isAdmin ? '/channels' : '/wallet',
      },
    ],
    [isAdmin, t]
  )

  const showSupplementalPanels = showFAQPanel || showUptimePanel

  return (
    <div className='flex flex-col gap-4'>
      <SummaryCards />

      <ConnectionOverview
        apiAddress={apiAddress}
        keyItem={preferredKey}
        keysLoading={apiKeysQuery.isLoading}
        actions={quickActions}
      />

      <div
        data-testid='overview-main-insights'
        className='grid min-w-0 grid-cols-1 gap-4 xl:grid-cols-3'
      >
        <div
          data-testid='overview-usage-trend'
          className={cn(
            'min-w-0',
            showAnnouncementsPanel ? 'xl:col-span-2' : 'xl:col-span-3'
          )}
        >
          <UsageTrendPanel />
        </div>
        {showAnnouncementsPanel && (
          <div
            data-testid='overview-announcements'
            className='min-w-0 xl:col-span-1'
          >
            <AnnouncementsPanel />
          </div>
        )}
      </div>

      {showSupplementalPanels && (
        <div
          className={cn(
            'grid grid-cols-1 gap-4',
            showFAQPanel && showUptimePanel && 'lg:grid-cols-2'
          )}
        >
          {showFAQPanel && <FAQPanel />}
          {showUptimePanel && <UptimePanel />}
        </div>
      )}
    </div>
  )
}
