import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import {
  ArrowRight,
  BookOpen,
  Check,
  Circle,
  Copy,
  CreditCard,
  FileText,
  KeyRound,
  RadioTower,
  TerminalSquare,
  type LucideIcon,
} from 'lucide-react'
import { type ReactNode, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { IconBadge, type IconBadgeTone } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { fetchTokenKey, getApiKeys } from '@/features/keys/api'
import type { ApiKey } from '@/features/keys/types'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { getUserModels } from '@/lib/api'
import { ROLE } from '@/lib/roles'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import { useDashboardStatus } from '../../hooks/use-status-data'
import { resolvePrimaryApiAddress } from '../../lib/api-info'
import { AnnouncementsPanel } from './announcements-panel'
import { FAQPanel } from './faq-panel'
import { SummaryCards } from './summary-cards'
import { UptimePanel } from './uptime-panel'

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
  icon: LucideIcon
  tone: IconBadgeTone
  adminOnly?: boolean
}

interface SetupStep {
  title: string
  description: string
  to: DashboardActionPath
  icon: LucideIcon
  completed: boolean
}

function getCurrentOrigin(): string {
  if (typeof window === 'undefined') return ''
  return window.location.origin
}

function normalizeRequestEndpoint(sourceUrl?: string): string {
  const fallback = `${getCurrentOrigin()}/v1/chat/completions`
  const trimmed = sourceUrl?.trim()
  if (!trimmed) return fallback

  const withoutTrailingSlash = trimmed.replace(/\/+$/, '')
  if (withoutTrailingSlash.endsWith('/v1/chat/completions')) {
    return withoutTrailingSlash
  }
  if (withoutTrailingSlash.endsWith('/v1')) {
    return `${withoutTrailingSlash}/chat/completions`
  }
  return `${withoutTrailingSlash}/v1/chat/completions`
}

function getPreferredKey(keys: ApiKey[]): ApiKey | null {
  return keys.find((item) => item.status === 1) ?? keys[0] ?? null
}

function formatMaskedKey(key: string): string {
  const fullKey = key.startsWith('sk-') ? key : `sk-${key}`
  if (fullKey.length <= 16) return fullKey
  return `${fullKey.slice(0, 7)}...${fullKey.slice(-4)}`
}

function buildCurlCommand(args: {
  endpoint: string
  apiKey: string
  model: string
}): string {
  return [
    `curl ${args.endpoint} \\`,
    '  -H "Content-Type: application/json" \\',
    `  -H "Authorization: Bearer ${args.apiKey}" \\`,
    `  -d '{"model":"${args.model}","messages":[{"role":"user","content":"Say hello in one sentence."}]}'`,
  ].join('\n')
}

function QuickActionButton({ action }: { action: QuickAction }) {
  const Icon = action.icon
  return (
    <Button
      variant='ghost'
      className='hover:bg-muted/60 h-auto min-w-0 justify-start gap-2.5 rounded-xl px-3 py-2.5'
      render={<Link to={action.to} />}
    >
      <IconBadge tone={action.tone} size='sm'>
        <Icon />
      </IconBadge>
      <span className='truncate text-sm'>{action.title}</span>
    </Button>
  )
}

function ConnectionOverview(props: {
  apiAddress: string
  keyItem: ApiKey | null
  keysLoading: boolean
  actions: QuickAction[]
}) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard({ notify: false })

  const handleCopyAddress = async () => {
    const copied = await copyToClipboard(props.apiAddress)
    if (copied) {
      toast.success(t('Copied to clipboard'))
    } else {
      toast.error(t('Failed to copy to clipboard'))
    }
  }

  let keyStatusContent: ReactNode
  if (props.keysLoading) {
    keyStatusContent = <Skeleton className='mt-3 h-5 w-32' />
  } else if (props.keyItem) {
    keyStatusContent = (
      <>
        <div className='mt-3 flex items-center gap-2'>
          <span
            className={cn(
              'size-2 rounded-full',
              props.keyItem.status === 1 ? 'bg-success' : 'bg-muted-foreground'
            )}
          />
          <span className='truncate text-sm font-semibold'>
            {props.keyItem.name}
          </span>
          <span className='text-muted-foreground ml-auto truncate font-mono text-xs'>
            {formatMaskedKey(props.keyItem.key)}
          </span>
        </div>
        <Button
          variant='link'
          size='sm'
          className='mt-1 h-auto p-0 text-xs'
          render={<Link to='/keys' />}
        >
          {t('Manage API Keys')}
          <ArrowRight data-icon='inline-end' />
        </Button>
      </>
    )
  } else {
    keyStatusContent = (
      <>
        <p className='mt-3 text-sm font-semibold'>{t('No API key yet')}</p>
        <Button
          variant='link'
          size='sm'
          className='mt-1 h-auto p-0 text-xs'
          render={<Link to='/keys' />}
        >
          {t('Create API Key')}
          <ArrowRight data-icon='inline-end' />
        </Button>
      </>
    )
  }

  return (
    <section className='bg-card overflow-hidden rounded-2xl border'>
      <div className='grid xl:grid-cols-[minmax(0,1fr)_minmax(22rem,0.72fr)]'>
        <div className='bg-border grid gap-px sm:grid-cols-2'>
          <div className='bg-card min-w-0 p-4 sm:p-5'>
            <div className='flex items-center justify-between gap-3'>
              <div className='flex min-w-0 items-center gap-2'>
                <IconBadge tone='info' size='sm'>
                  <RadioTower />
                </IconBadge>
                <span className='text-muted-foreground text-xs font-medium'>
                  {t('API address')}
                </span>
              </div>
              <Button
                variant='ghost'
                size='icon-sm'
                onClick={handleCopyAddress}
                aria-label={t('Copy API address')}
              >
                <Copy />
              </Button>
            </div>
            <p
              className='mt-3 truncate font-mono text-sm font-semibold'
              title={props.apiAddress}
            >
              {props.apiAddress}
            </p>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t('Use this base URL in your API client')}
            </p>
          </div>

          <div className='bg-card min-w-0 p-4 sm:p-5'>
            <div className='flex items-center gap-2'>
              <IconBadge
                tone={props.keyItem?.status === 1 ? 'success' : 'warning'}
                size='sm'
              >
                <KeyRound />
              </IconBadge>
              <span className='text-muted-foreground text-xs font-medium'>
                {t('API key status')}
              </span>
            </div>
            {keyStatusContent}
          </div>
        </div>

        <div className='border-t p-3 sm:p-4 xl:border-t-0 xl:border-l'>
          <p className='text-muted-foreground px-2 pb-2 text-xs font-medium'>
            {t('Quick actions')}
          </p>
          <div className='grid grid-cols-2 gap-1'>
            {props.actions.map((action) => (
              <QuickActionButton key={action.title} action={action} />
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}

function SetupGuide(props: {
  steps: SetupStep[]
  keyItem: ApiKey | null
  model: string
  endpoint: string
}) {
  const { t } = useTranslation()
  const [isCopying, setIsCopying] = useState(false)
  const { copyToClipboard } = useCopyToClipboard({ notify: false })

  const handleCopyRequest = async () => {
    if (!props.keyItem || isCopying) return

    setIsCopying(true)
    try {
      const result = await fetchTokenKey(props.keyItem.id)
      const key = result.success && result.data?.key ? result.data.key : ''
      if (!key) {
        toast.error(result.message || t('Failed to copy to clipboard'))
        return
      }
      const copied = await copyToClipboard(
        buildCurlCommand({
          endpoint: props.endpoint,
          apiKey: `sk-${key}`,
          model: props.model,
        })
      )
      if (copied) {
        toast.success(t('Copied to clipboard'))
      } else {
        toast.error(t('Failed to copy to clipboard'))
      }
    } finally {
      setIsCopying(false)
    }
  }

  const previewKey = props.keyItem
    ? formatMaskedKey(props.keyItem.key)
    : 'sk-...'
  const preview = buildCurlCommand({
    endpoint: props.endpoint,
    apiKey: previewKey,
    model: props.model,
  })

  return (
    <section className='bg-card overflow-hidden rounded-2xl border'>
      <div className='grid lg:grid-cols-[minmax(0,0.82fr)_minmax(24rem,1.18fr)]'>
        <div className='min-w-0 p-4 sm:p-5'>
          <p className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
            {t('Get started')}
          </p>
          <h2 className='mt-1 text-lg font-semibold tracking-tight'>
            {t('Complete your first API request')}
          </h2>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t('The setup guide disappears after your first successful call.')}
          </p>

          <ol className='mt-4 space-y-1'>
            {props.steps.map((step) => {
              const Icon = step.icon
              const StatusIcon = step.completed ? Check : Circle
              return (
                <li key={step.title}>
                  <Link
                    to={step.to}
                    className='hover:bg-muted/50 focus-visible:ring-ring flex items-center gap-3 rounded-xl px-2 py-2.5 outline-none focus-visible:ring-2'
                  >
                    <span
                      className={cn(
                        'flex size-8 shrink-0 items-center justify-center rounded-lg',
                        step.completed ? 'bg-success/10' : 'bg-muted'
                      )}
                    >
                      <StatusIcon
                        className={cn(
                          'size-4',
                          step.completed && 'text-success'
                        )}
                      />
                    </span>
                    <span className='min-w-0 flex-1'>
                      <span className='flex items-center gap-2 text-sm font-medium'>
                        <Icon className='size-3.5' />
                        {step.title}
                      </span>
                      <span className='text-muted-foreground line-clamp-1 text-xs'>
                        {step.description}
                      </span>
                    </span>
                    <ArrowRight className='text-muted-foreground size-4' />
                  </Link>
                </li>
              )
            })}
          </ol>
        </div>

        <div className='bg-muted/35 min-w-0 border-t p-4 sm:p-5 lg:border-t-0 lg:border-l'>
          <div className='flex items-center justify-between gap-3'>
            <div>
              <p className='text-sm font-semibold'>{t('First API request')}</p>
              <p className='text-muted-foreground text-xs'>
                {props.keyItem?.name || t('Create an API key to continue')}
              </p>
            </div>
            {props.keyItem ? (
              <Button
                variant='outline'
                size='sm'
                onClick={handleCopyRequest}
                disabled={isCopying}
              >
                <Copy data-icon='inline-start' />
                {isCopying ? t('Loading') : t('Copy curl')}
              </Button>
            ) : (
              <Button size='sm' render={<Link to='/keys' />}>
                <KeyRound data-icon='inline-start' />
                {t('Create API Key')}
              </Button>
            )}
          </div>
          <pre className='bg-background mt-4 max-h-44 overflow-auto rounded-xl border p-3 text-xs leading-relaxed'>
            <code>{preview}</code>
          </pre>
        </div>
      </div>
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

  const requestCount = Number(user?.request_count ?? 0)
  const remainQuota = Number(user?.quota ?? 0)
  const usedQuota = Number(user?.used_quota ?? 0)
  const isAdmin = Boolean(user?.role && user.role >= ROLE.ADMIN)

  const apiKeysQuery = useQuery({
    queryKey: ['dashboard', 'overview', 'api-keys'],
    queryFn: async () => {
      const result = await getApiKeys({ p: 1, size: 10 })
      return result.success ? (result.data?.items ?? []) : []
    },
    staleTime: 60 * 1000,
  })

  const modelsQuery = useQuery({
    queryKey: ['dashboard', 'overview', 'user-models'],
    queryFn: async () => {
      const result = await getUserModels()
      return result.success ? (result.data ?? []) : []
    },
    enabled: requestCount === 0,
    staleTime: 5 * 60 * 1000,
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
        icon: KeyRound,
        tone: 'info',
      },
      {
        title: t('Usage Logs'),
        to: '/usage-logs',
        icon: FileText,
        tone: 'chart-4',
      },
      {
        title: t('Pricing'),
        to: '/pricing',
        icon: BookOpen,
        tone: 'warning',
      },
      {
        title: isAdmin ? t('Channels') : t('Wallet'),
        to: isAdmin ? '/channels' : '/wallet',
        icon: isAdmin ? RadioTower : CreditCard,
        tone: 'success',
        adminOnly: isAdmin,
      },
    ],
    [isAdmin, t]
  )

  const setupSteps = useMemo<SetupStep[]>(
    () => [
      {
        title: t('Create API Key'),
        description: t('Create a key for your app or service'),
        to: '/keys',
        icon: KeyRound,
        completed: Boolean(preferredKey),
      },
      {
        title: t('Add credits'),
        description: t('Keep enough balance before production traffic'),
        to: '/wallet',
        icon: CreditCard,
        completed: remainQuota > 0 || usedQuota > 0,
      },
      {
        title: t('Send a request'),
        description: t('Verify routing with Playground or your client'),
        to: '/playground',
        icon: TerminalSquare,
        completed: requestCount > 0,
      },
    ],
    [preferredKey, remainQuota, requestCount, t, usedQuota]
  )

  const showSetupGuide =
    apiKeysQuery.isFetched &&
    Boolean(user) &&
    (!preferredKey || requestCount === 0)
  const showSecondaryPanels =
    showAnnouncementsPanel || showFAQPanel || showUptimePanel

  return (
    <div className='flex flex-col gap-4'>
      <SummaryCards />

      <ConnectionOverview
        apiAddress={apiAddress}
        keyItem={preferredKey}
        keysLoading={apiKeysQuery.isLoading}
        actions={quickActions}
      />

      {showSetupGuide && (
        <SetupGuide
          steps={setupSteps}
          keyItem={preferredKey}
          model={modelsQuery.data?.[0] ?? 'gpt-4o-mini'}
          endpoint={normalizeRequestEndpoint(apiAddress)}
        />
      )}

      {showSecondaryPanels && (
        <div className='grid grid-cols-1 gap-4 xl:grid-cols-3'>
          {showAnnouncementsPanel && <AnnouncementsPanel />}
          {showFAQPanel && <FAQPanel />}
          {showUptimePanel && <UptimePanel />}
        </div>
      )}
    </div>
  )
}
