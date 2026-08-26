import { useQuery } from '@tanstack/react-query'
import {
  Activity,
  CircleAlert,
  Crown,
  Flame,
  WalletCards,
  type LucideIcon,
} from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { IconBadge, type IconBadgeTone } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { getUserQuotaDates } from '@/features/dashboard/api'
import {
  getErrorRate,
  getRemainingSubscriptionQuota,
} from '@/features/dashboard/lib/overview-metrics'
import { getPerfMetricsSummary } from '@/features/performance-metrics/api'
import { getSelfSubscriptionFull } from '@/features/subscriptions/api'
import { formatNumber, formatQuota } from '@/lib/format'
import { computeTimeRange } from '@/lib/time'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

interface SummaryItem {
  key: string
  title: string
  value: string
  description: string
  icon: LucideIcon
  tone: IconBadgeTone
  loading: boolean
  valueClassName?: string
}

export function SummaryCards() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const timeRange = useMemo(() => computeTimeRange(1), [])

  const usageQuery = useQuery({
    queryKey: [
      'dashboard',
      'overview',
      'last-24h-usage',
      timeRange.start_timestamp,
      timeRange.end_timestamp,
    ],
    queryFn: () =>
      getUserQuotaDates({
        start_timestamp: timeRange.start_timestamp,
        end_timestamp: timeRange.end_timestamp,
        default_time: 'hour',
      }),
    staleTime: 60 * 1000,
  })

  const performanceQuery = useQuery({
    queryKey: ['dashboard', 'overview', 'error-rate', 24],
    queryFn: () => getPerfMetricsSummary(24),
    staleTime: 60 * 1000,
    retry: false,
  })

  const subscriptionQuery = useQuery({
    queryKey: ['dashboard', 'overview', 'subscription-quota', user?.id],
    queryFn: getSelfSubscriptionFull,
    enabled: Boolean(user),
    staleTime: 60 * 1000,
    retry: false,
  })

  const recentUsage = useMemo(
    () =>
      (usageQuery.data?.data ?? []).reduce(
        (total, item) => total + (Number(item.quota) || 0),
        0
      ),
    [usageQuery.data?.data]
  )
  const errorRate = getErrorRate(performanceQuery.data?.data.models ?? [])
  const remainingSubscriptionQuota = getRemainingSubscriptionQuota(
    subscriptionQuery.data?.data?.subscriptions ?? []
  )
  const errorRateDisplay = errorRate === null ? '—' : `${errorRate.toFixed(2)}%`
  let errorRateClass: string | undefined
  if (errorRate !== null && errorRate >= 10) {
    errorRateClass = 'text-destructive'
  } else if (errorRate !== null && errorRate >= 5) {
    errorRateClass = 'text-warning'
  }

  const items: SummaryItem[] = [
    {
      key: 'balance',
      title: t('Credit remaining'),
      value: formatQuota(Number(user?.quota ?? 0)),
      description: t('Available balance for future requests'),
      icon: WalletCards,
      tone: 'success',
      loading: !user,
    },
    {
      key: 'usage',
      title: t('Last 24h usage'),
      value: formatQuota(recentUsage),
      description: t('Consumed in the last 24 hours'),
      icon: Flame,
      tone: 'warning',
      loading: usageQuery.isLoading,
    },
    {
      key: 'subscription',
      title: t('Subscription'),
      value: subscriptionQuery.isError
        ? '—'
        : formatQuota(remainingSubscriptionQuota),
      description: t('Remaining quota'),
      icon: Crown,
      tone: 'primary',
      loading: !user || subscriptionQuery.isLoading,
    },
    {
      key: 'requests',
      title: t('Request Count'),
      value: formatNumber(Number(user?.request_count ?? 0)),
      description: t('Total requests made'),
      icon: Activity,
      tone: 'info',
      loading: !user,
    },
    {
      key: 'errors',
      title: t('Error rate'),
      value: errorRateDisplay,
      description: t('Across active models in the last 24 hours'),
      icon: CircleAlert,
      tone: errorRate !== null && errorRate >= 5 ? 'warning' : 'success',
      loading: performanceQuery.isLoading,
      valueClassName: errorRateClass,
    },
  ]

  return (
    <section
      aria-label={t('Account and traffic summary')}
      className='bg-border grid gap-px overflow-hidden rounded-2xl border sm:grid-cols-2 xl:grid-cols-5'
    >
      {items.map((item) => {
        const Icon = item.icon
        return (
          <div
            key={item.key}
            role='group'
            aria-label={item.title}
            className='bg-card min-w-0 p-4 sm:p-5'
          >
            <div className='flex items-start justify-between gap-3'>
              <div className='min-w-0'>
                <p className='text-muted-foreground text-xs font-medium'>
                  {item.title}
                </p>
                {item.loading ? (
                  <Skeleton className='mt-2 h-8 w-28' />
                ) : (
                  <p
                    className={cn(
                      'mt-1.5 font-mono text-2xl font-semibold tracking-tight tabular-nums',
                      item.valueClassName
                    )}
                  >
                    {item.value}
                  </p>
                )}
              </div>
              <IconBadge tone={item.tone} size='sm'>
                <Icon />
              </IconBadge>
            </div>
            <p className='text-muted-foreground mt-3 truncate text-xs'>
              {item.description}
            </p>
          </div>
        )
      })}
    </section>
  )
}
