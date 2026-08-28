import { Crown, RefreshCw, Sparkles, Check } from 'lucide-react'
import { useState, useEffect, useMemo, useCallback } from 'react'
import { useTranslation } from 'react-i18next'

import {
  StatusBadge,
  dotColorMap,
  textColorMap,
} from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  getPublicPlans,
  getSelfSubscriptionFull,
} from '@/features/subscriptions/api'
import { SubscriptionPurchaseDialog } from '@/features/subscriptions/components/dialogs/subscription-purchase-dialog'
import { formatSubscriptionPrice } from '@/features/subscriptions/lib'
import type {
  PublicPlanRecord,
  StripeInvoiceSummary,
  StripeSubscriptionSummary,
  UserSubscriptionRecord,
} from '@/features/subscriptions/types'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { isStripeSubscriptionEnabled } from '../lib/payment'
import type { PaymentMethod, TopupInfo } from '../types'
import { StripeSubscriptionBilling } from './stripe-subscription-billing'

interface SubscriptionPlansCardProps {
  topupInfo: TopupInfo | null
}

const subscriptionPlansGridClassName =
  'grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3 md:gap-5'

function getEpayMethods(payMethods: PaymentMethod[] = []): PaymentMethod[] {
  return payMethods.filter(
    (m) => m?.type && m.type !== 'stripe' && m.type !== 'creem'
  )
}

export function SubscriptionPlansCard({
  topupInfo,
}: SubscriptionPlansCardProps) {
  const { t } = useTranslation()
  const configuredQuotaPerUnit = useSystemConfigStore(
    (state) => state.config.currency.quotaPerUnit
  )
  const quotaPerUnit =
    Number.isFinite(configuredQuotaPerUnit) && configuredQuotaPerUnit > 0
      ? configuredQuotaPerUnit
      : DEFAULT_CURRENCY_CONFIG.quotaPerUnit

  const [plans, setPlans] = useState<PublicPlanRecord[]>([])
  const [activeSubscriptions, setActiveSubscriptions] = useState<
    UserSubscriptionRecord[]
  >([])
  const [allSubscriptions, setAllSubscriptions] = useState<
    UserSubscriptionRecord[]
  >([])
  const [stripeSubscriptions, setStripeSubscriptions] = useState<
    StripeSubscriptionSummary[]
  >([])
  const [stripeInvoices, setStripeInvoices] = useState<StripeInvoiceSummary[]>(
    []
  )
  const [billingDebt, setBillingDebt] = useState(0)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)

  const [purchaseOpen, setPurchaseOpen] = useState(false)
  const [selectedPlan, setSelectedPlan] = useState<PublicPlanRecord | null>(
    null
  )

  const enableStripe = isStripeSubscriptionEnabled(topupInfo)
  const enableCreem = !!topupInfo?.enable_creem_topup
  const enableWaffoPancake = !!topupInfo?.enable_waffo_pancake_topup
  const enableOnlineTopUp = !!topupInfo?.enable_online_topup
  const epayMethods = useMemo(
    () => getEpayMethods(topupInfo?.pay_methods),
    [topupInfo?.pay_methods]
  )

  const fetchPlans = useCallback(async () => {
    try {
      const res = await getPublicPlans()
      if (res.success) {
        setPlans(res.data || [])
      }
    } catch {
      setPlans([])
    }
  }, [])

  const fetchSelfSubscription = useCallback(async () => {
    try {
      const res = await getSelfSubscriptionFull()
      if (res.success && res.data) {
        setActiveSubscriptions(res.data.subscriptions || [])
        setAllSubscriptions(res.data.all_subscriptions || [])
        setStripeSubscriptions(res.data.stripe_subscriptions || [])
        setStripeInvoices(res.data.stripe_invoices || [])
        setBillingDebt(Number(res.data.billing_debt || 0))
      }
    } catch {
      // ignore
    }
  }, [])

  useEffect(() => {
    const init = async () => {
      setLoading(true)
      await Promise.all([fetchPlans(), fetchSelfSubscription()])
      setLoading(false)
    }
    init()
  }, [fetchPlans, fetchSelfSubscription])

  const handleRefresh = async () => {
    setRefreshing(true)
    try {
      await fetchSelfSubscription()
    } finally {
      setRefreshing(false)
    }
  }

  const hasActive = activeSubscriptions.length > 0
  const hasAny = allSubscriptions.length > 0

  const activePlanIds = useMemo(() => {
    const ids = new Set<number>()
    for (const sub of activeSubscriptions) {
      const planId = sub?.subscription?.plan_id
      if (planId) ids.add(planId)
    }
    return ids
  }, [activeSubscriptions])

  const planPurchaseCountMap = useMemo(() => {
    const map = new Map<number, number>()
    for (const sub of allSubscriptions) {
      const planId = sub?.subscription?.plan_id
      if (!planId) continue
      map.set(planId, (map.get(planId) || 0) + 1)
    }
    return map
  }, [allSubscriptions])

  const planTitleMap = useMemo(() => {
    const map = new Map<number, string>()
    for (const p of plans) {
      if (p?.plan?.id) {
        map.set(p.plan.id, p.plan.title || '')
      }
    }
    return map
  }, [plans])

  const planDiscountMap = useMemo(() => {
    const discountByPlanId = new Map<number, number>()
    for (const record of plans) {
      const plan = record?.plan
      const priceAmount = Number(plan?.price_amount || 0)
      const totalAmount = Number(plan?.total_amount || 0)
      if (
        !plan ||
        !Number.isFinite(priceAmount) ||
        !Number.isFinite(totalAmount) ||
        priceAmount <= 0 ||
        totalAmount <= 0
      ) {
        continue
      }

      const monthlyQuota = totalAmount / quotaPerUnit
      const discount = Math.floor((priceAmount / monthlyQuota) * 100) / 10
      if (discount > 0 && discount < 10) {
        discountByPlanId.set(plan.id, discount)
      }
    }

    return discountByPlanId
  }, [plans, quotaPerUnit])

  const getRemainingDays = (sub: UserSubscriptionRecord) => {
    const endTime = sub?.subscription?.end_time || 0
    if (!endTime) return 0
    const now = Date.now() / 1000
    return Math.max(0, Math.ceil((endTime - now) / 86400))
  }

  const getUsagePercent = (sub: UserSubscriptionRecord) => {
    const total = Number(sub?.subscription?.amount_total || 0)
    const used = Number(sub?.subscription?.amount_used || 0)
    if (total <= 0) return 0
    return Math.round((used / total) * 100)
  }

  if (loading) {
    return (
      <Card data-card-hover='false' className='gap-0 overflow-hidden py-0'>
        <CardHeader className='border-b p-3 !pb-3 sm:p-5 sm:!pb-5'>
          <Skeleton className='h-6 w-32' />
        </CardHeader>
        <CardContent className='space-y-4 p-3 sm:p-5'>
          <Skeleton className='h-20 w-full' />
          <div className={subscriptionPlansGridClassName}>
            {['first', 'second', 'third'].map((key) => (
              <Skeleton key={key} className='h-[340px] w-full' />
            ))}
          </div>
        </CardContent>
      </Card>
    )
  }

  if (plans.length === 0 && !hasAny) {
    return null
  }

  return (
    <>
      <TitledCard
        title={t('Subscription Plans')}
        description={t('Subscribe to a plan for model access')}
        icon={<Crown className='h-4 w-4' />}
        iconTone='warning'
        disableHoverEffect
        contentClassName='space-y-4 sm:space-y-5'
      >
        {/* My subscriptions */}
        {hasAny && (
          <div className='rounded-xl border p-3 sm:p-4'>
            <div className='flex flex-wrap items-center justify-between gap-2.5 sm:gap-3'>
              <div className='flex min-w-0 flex-wrap items-center gap-2'>
                <span className='text-sm font-medium'>
                  {t('My Subscriptions')}
                </span>
                <span className='flex items-center gap-1.5 text-xs font-medium'>
                  <span
                    className={cn(
                      'size-1.5 shrink-0 rounded-full',
                      hasActive ? dotColorMap.success : dotColorMap.neutral
                    )}
                    aria-hidden='true'
                  />
                  {hasActive ? (
                    <span className={cn(textColorMap.success)}>
                      {activeSubscriptions.length} {t('active')}
                    </span>
                  ) : (
                    <span className='text-muted-foreground'>
                      {t('No Active')}
                    </span>
                  )}
                  {allSubscriptions.length > activeSubscriptions.length && (
                    <>
                      <span className='text-muted-foreground/30'>·</span>
                      <span className='text-muted-foreground'>
                        {allSubscriptions.length - activeSubscriptions.length}{' '}
                        {t('expired')}
                      </span>
                    </>
                  )}
                </span>
              </div>
              <div className='flex items-center'>
                <Button
                  variant='ghost'
                  size='icon'
                  className='h-8 w-8'
                  onClick={handleRefresh}
                  disabled={refreshing}
                  aria-label={t('Refresh subscriptions')}
                >
                  <RefreshCw
                    className={`h-3.5 w-3.5 ${refreshing ? 'animate-spin' : ''}`}
                    aria-hidden='true'
                  />
                </Button>
              </div>
            </div>

            <Separator className='my-3' />
            <div className='max-h-64 space-y-3 overflow-y-auto pr-1'>
              {allSubscriptions.map((sub) => {
                const subscription = sub.subscription
                const totalAmount = Number(subscription?.amount_total || 0)
                const usedAmount = Number(subscription?.amount_used || 0)
                const remainAmount =
                  totalAmount > 0 ? Math.max(0, totalAmount - usedAmount) : 0
                const planTitle = planTitleMap.get(subscription?.plan_id) || ''
                const remainDays = getRemainingDays(sub)
                const usagePercent = getUsagePercent(sub)
                const now = Date.now() / 1000
                const isExpired = (subscription?.end_time || 0) < now
                const isCancelled = subscription?.status === 'cancelled'
                const isActive = subscription?.status === 'active' && !isExpired
                const nextResetTime = subscription?.next_reset_time ?? 0
                let statusBadge = (
                  <StatusBadge
                    label={t('Expired')}
                    variant='neutral'
                    copyable={false}
                  />
                )
                if (isActive) {
                  statusBadge = (
                    <StatusBadge
                      label={t('Active')}
                      variant='success'
                      copyable={false}
                    />
                  )
                } else if (isCancelled) {
                  statusBadge = (
                    <StatusBadge
                      label={t('Cancelled')}
                      variant='neutral'
                      copyable={false}
                    />
                  )
                }

                let endTimeLabel = t('Expired at')
                if (isActive) {
                  endTimeLabel = t('Until')
                } else if (isCancelled) {
                  endTimeLabel = t('Cancelled at')
                }

                return (
                  <div
                    key={subscription?.id}
                    className='bg-background rounded-md border p-3 text-xs'
                  >
                    <div className='flex items-center justify-between'>
                      <div className='flex items-center gap-2'>
                        <span className='font-medium'>
                          {planTitle
                            ? `${planTitle} · ${t('Subscription')} #${subscription?.id}`
                            : `${t('Subscription')} #${subscription?.id}`}
                        </span>
                        {statusBadge}
                      </div>
                      {isActive && (
                        <span className='text-muted-foreground'>
                          {t('{{count}} days remaining', {
                            count: remainDays,
                          })}
                        </span>
                      )}
                    </div>
                    <div className='text-muted-foreground mt-1.5'>
                      {endTimeLabel}{' '}
                      {new Date(
                        (subscription?.end_time || 0) * 1000
                      ).toLocaleString()}
                    </div>
                    {isActive && nextResetTime > 0 && (
                      <div className='text-muted-foreground mt-1'>
                        {t('Next reset')}:{' '}
                        {new Date(nextResetTime * 1000).toLocaleString()}
                      </div>
                    )}
                    <div className='text-muted-foreground mt-1'>
                      {t('Monthly Quota')}:{' '}
                      {totalAmount > 0 ? (
                        <Tooltip>
                          <TooltipTrigger
                            render={<span className='cursor-help' />}
                          >
                            {formatQuota(usedAmount)}/{formatQuota(totalAmount)}{' '}
                            · {t('Remaining')} {formatQuota(remainAmount)}
                          </TooltipTrigger>
                          <TooltipContent>
                            {t('Raw Quota')}: {usedAmount}/{totalAmount} ·{' '}
                            {t('Remaining')} {remainAmount}
                          </TooltipContent>
                        </Tooltip>
                      ) : (
                        t('Unlimited')
                      )}
                      {totalAmount > 0 && (
                        <span className='ml-2'>
                          {t('Used')} {usagePercent}%
                        </span>
                      )}
                    </div>
                    {totalAmount > 0 && isActive && (
                      <Progress value={usagePercent} className='mt-2 h-1.5' />
                    )}
                  </div>
                )
              })}
            </div>
          </div>
        )}

        {hasAny && (
          <StripeSubscriptionBilling
            subscriptions={stripeSubscriptions}
            invoices={stripeInvoices}
            billingDebt={billingDebt}
            onRefresh={fetchSelfSubscription}
          />
        )}

        {hasActive && plans.length > 0 && (
          <p
            className='bg-muted/30 text-muted-foreground rounded-lg border px-3 py-2 text-sm leading-5'
            role='status'
          >
            {t(
              'Plan changes are not supported while you have an active subscription.'
            )}
          </p>
        )}

        {/* Available plans grid */}
        {plans.length > 0 ? (
          <div
            data-slot='subscription-plans-grid'
            className={subscriptionPlansGridClassName}
          >
            {plans.map((p) => {
              const plan = p?.plan
              if (!plan) return null
              const totalAmount = Number(plan.total_amount || 0)
              const price = formatSubscriptionPrice(
                Number(plan.price_amount || 0),
                plan.currency
              )
              const isPopular = plan.recommended
              const discount = planDiscountMap.get(plan.id)
              const limit = Number(plan.max_purchase_per_user || 0)
              const count = planPurchaseCountMap.get(plan.id) || 0
              const reached = limit > 0 && count >= limit
              const isCurrentPlan = activePlanIds.has(plan.id)
              const purchaseAvailable =
                (enableStripe && plan.stripe_checkout_available) ||
                (enableCreem && plan.creem_checkout_available) ||
                (enableWaffoPancake && plan.waffo_checkout_available) ||
                (enableOnlineTopUp && epayMethods.length > 0)
              const quota =
                totalAmount > 0 ? formatQuota(totalAmount) : t('Unlimited')

              const benefits = [
                t('Monthly billing'),
                t('Monthly quota {{quota}}', { quota }),
                t('Credits refresh with each monthly renewal'),
                limit > 0 ? `${t('Purchase Limit')}: ${limit}` : null,
                plan.upgrade_group
                  ? `${t('Upgrade Group')}: ${plan.upgrade_group}`
                  : null,
              ].filter(Boolean) as string[]

              return (
                <Card
                  key={plan.id}
                  data-card-hover='false'
                  className={cn(
                    'min-h-[340px] shadow-sm',
                    isPopular &&
                      'border-primary/70 ring-primary/50 shadow-md ring-2'
                  )}
                >
                  <CardContent className='flex flex-1 flex-col p-5 sm:p-6'>
                    <div className='mb-4 flex items-start justify-between gap-3'>
                      <div className='min-w-0'>
                        <h4 className='truncate text-lg leading-tight font-semibold'>
                          {plan.title || t('Subscription Plans')}
                        </h4>
                        {plan.subtitle && (
                          <p className='text-muted-foreground mt-1 line-clamp-2 min-h-10 text-sm leading-5'>
                            {plan.subtitle}
                          </p>
                        )}
                      </div>
                      {isPopular && (
                        <StatusBadge
                          variant='info'
                          copyable={false}
                          className='shrink-0'
                        >
                          <Sparkles className='h-3 w-3' />
                          {t('Recommended')}
                        </StatusBadge>
                      )}
                    </div>

                    <div className='min-h-6'>
                      {discount !== undefined && (
                        <StatusBadge
                          label={t('{{discount}}/10 price', { discount })}
                          variant='success'
                          copyable={false}
                        />
                      )}
                    </div>

                    <div className='py-3'>
                      <span className='text-primary text-3xl font-bold'>
                        {price}
                      </span>
                    </div>

                    <div className='flex flex-1 flex-col gap-2.5 pb-5'>
                      {benefits.map((label) => (
                        <div
                          key={label}
                          className='text-muted-foreground flex items-center gap-2 text-sm'
                        >
                          <Check className='text-primary size-4 shrink-0' />
                          <span>{label}</span>
                        </div>
                      ))}
                    </div>

                    <Separator className='mb-4' />

                    {hasActive && (
                      <Tooltip>
                        <TooltipTrigger render={<div />}>
                          <Button
                            variant='outline'
                            size='lg'
                            className='w-full'
                            disabled
                          >
                            {isCurrentPlan
                              ? t('Current plan')
                              : t('Plan change unavailable')}
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>
                          {t(
                            'Plan changes are not supported while you have an active subscription.'
                          )}
                        </TooltipContent>
                      </Tooltip>
                    )}

                    {!hasActive && reached && (
                      <Tooltip>
                        <TooltipTrigger render={<div />}>
                          <Button
                            variant='outline'
                            size='lg'
                            className='w-full'
                            disabled
                          >
                            {t('Limit Reached')}
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>
                          {t('Purchase limit reached')} ({count}/{limit})
                        </TooltipContent>
                      </Tooltip>
                    )}

                    {!hasActive && !reached && !purchaseAvailable && (
                      <Button
                        variant='outline'
                        size='lg'
                        className='w-full'
                        disabled
                      >
                        {t('Not available')}
                      </Button>
                    )}

                    {!hasActive && !reached && purchaseAvailable && (
                      <Button
                        variant='outline'
                        size='lg'
                        className='w-full'
                        onClick={() => {
                          setSelectedPlan(p)
                          setPurchaseOpen(true)
                        }}
                      >
                        {t('Subscribe Now')}
                      </Button>
                    )}
                  </CardContent>
                </Card>
              )
            })}
          </div>
        ) : (
          <p className='text-muted-foreground py-4 text-center text-sm'>
            {t('No plans available')}
          </p>
        )}
      </TitledCard>

      <SubscriptionPurchaseDialog
        open={purchaseOpen}
        onOpenChange={(open) => {
          setPurchaseOpen(open)
          if (!open) {
            fetchSelfSubscription()
          }
        }}
        plan={selectedPlan}
        enableStripe={enableStripe}
        enableCreem={enableCreem}
        enableWaffoPancake={enableWaffoPancake}
        enableOnlineTopUp={enableOnlineTopUp}
        epayMethods={epayMethods}
        purchaseLimit={
          selectedPlan?.plan?.max_purchase_per_user
            ? Number(selectedPlan.plan.max_purchase_per_user)
            : undefined
        }
        purchaseCount={
          selectedPlan?.plan?.id
            ? planPurchaseCountMap.get(selectedPlan.plan.id)
            : undefined
        }
      />
    </>
  )
}
