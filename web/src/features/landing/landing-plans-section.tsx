import { Loading03Icon, Tick02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  getPublicPlans,
  paySubscriptionStripe,
} from '@/features/subscriptions/api'
import { formatSubscriptionPrice } from '@/features/subscriptions/lib'
import type { PublicSubscriptionPlan } from '@/features/subscriptions/types'
import { redirectToHostedCheckout } from '@/features/wallet/lib'
import { formatQuota } from '@/lib/format'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

interface LandingPlansSectionProps {
  isAuthenticated: boolean
  loadPlans?: typeof getPublicPlans
  createCheckout?: typeof paySubscriptionStripe
  redirectToCheckout?: typeof redirectToHostedCheckout
}

function PlanCard(props: {
  plan: PublicSubscriptionPlan
  quotaPerUnit: number
  isAuthenticated: boolean
  pending: boolean
  onSubscribe: (planId: number) => void
}) {
  const { t } = useTranslation()
  const totalAmount = Number(props.plan.total_amount || 0)
  const monthlyQuota = totalAmount / props.quotaPerUnit
  const quotaLabel =
    totalAmount > 0 && Number.isFinite(monthlyQuota)
      ? formatQuota(totalAmount)
      : t('Unlimited')
  const priceAmount = Number(props.plan.price_amount || 0)
  const discount =
    Number.isFinite(priceAmount) && priceAmount > 0 && monthlyQuota > 0
      ? Math.floor((priceAmount / monthlyQuota) * 100) / 10
      : null
  const signInHref = `/sign-in?redirect=${encodeURIComponent('/#plans')}`
  const actionClassName = `mt-6 h-11 w-full rounded-xl ${
    props.plan.recommended
      ? 'bg-[#5d3dff] text-white hover:bg-[#6c50ff]'
      : 'bg-white/12 text-white hover:bg-white/18'
  }`
  let planAction = (
    <Button
      className='mt-6 h-11 w-full rounded-xl bg-white/12 text-white'
      disabled
    >
      {t('Not available')}
    </Button>
  )

  if (props.plan.stripe_checkout_available) {
    if (props.isAuthenticated) {
      planAction = (
        <Button
          className={actionClassName}
          onClick={() => props.onSubscribe(props.plan.id)}
          disabled={props.pending}
        >
          {props.pending ? t('Opening checkout...') : t('Subscribe now')}
        </Button>
      )
    } else {
      planAction = (
        <Button className={actionClassName} render={<a href={signInHref} />}>
          {t('Sign in to subscribe')}
        </Button>
      )
    }
  }

  return (
    <article
      className={`relative flex min-h-[430px] flex-col rounded-[24px] border bg-[#181d25] p-6 shadow-2xl shadow-black/10 ${
        props.plan.recommended
          ? 'border-[#7d7bff] ring-1 ring-[#7d7bff]'
          : 'border-white/12'
      }`}
    >
      {props.plan.recommended && (
        <span className='absolute -top-3 left-1/2 -translate-x-1/2 rounded-full bg-[#6657ff] px-4 py-1 text-xs font-semibold whitespace-nowrap text-white'>
          {t('Recommended')}
        </span>
      )}

      <div>
        <h3 className='text-xl font-semibold text-white'>{props.plan.title}</h3>
        <p className='mt-1.5 min-h-6 text-sm text-white/55'>
          {props.plan.subtitle || null}
        </p>
      </div>

      <div className='mt-6 min-h-5'>
        {discount !== null && discount < 10 && (
          <Badge
            variant='warning'
            className='border-[#ef884c]/40 bg-[#ef884c]/10 text-[#ffb17f]'
          >
            {t('{{discount}}/10 price', { discount })}
          </Badge>
        )}
      </div>

      <div className='mt-2 flex items-end gap-1.5 border-b border-white/12 pb-5'>
        <span className='text-4xl font-bold tracking-[-0.04em] text-white'>
          {formatSubscriptionPrice(
            Number(props.plan.price_amount || 0),
            props.plan.currency
          )}
        </span>
        <span className='pb-1 text-sm text-white/55'>/{t('month')}</span>
      </div>

      <ul className='mt-5 flex-1 space-y-3 text-sm leading-6 text-white/78'>
        {[
          t('Monthly quota {{quota}}', { quota: quotaLabel }),
          t('Credits refresh with each monthly renewal'),
          t('Renews automatically every month'),
        ].map((benefit) => (
          <li key={benefit} className='flex items-start gap-3'>
            <HugeiconsIcon
              icon={Tick02Icon}
              strokeWidth={2}
              className={
                props.plan.recommended
                  ? 'mt-1 size-4 shrink-0 text-[#8584ff]'
                  : 'mt-1 size-4 shrink-0 text-white/55'
              }
              aria-hidden='true'
            />
            <span>{benefit}</span>
          </li>
        ))}
      </ul>

      {planAction}
    </article>
  )
}

export function LandingPlansSection(props: LandingPlansSectionProps) {
  const { t } = useTranslation()
  const configuredQuotaPerUnit = useSystemConfigStore(
    (state) => state.config.currency.quotaPerUnit
  )
  const quotaPerUnit =
    configuredQuotaPerUnit > 0
      ? configuredQuotaPerUnit
      : DEFAULT_CURRENCY_CONFIG.quotaPerUnit
  const loadPlans = props.loadPlans ?? getPublicPlans
  const createCheckout = props.createCheckout ?? paySubscriptionStripe
  const redirectToCheckout =
    props.redirectToCheckout ?? redirectToHostedCheckout
  const plansQuery = useQuery({
    queryKey: ['public-subscription-plans'],
    queryFn: async () => {
      const response = await loadPlans()
      if (!response.success) {
        throw new Error(response.message || 'Unable to load plans')
      }
      return response.data || []
    },
  })
  const landingPlans = plansQuery.data || []
  const checkoutMutation = useMutation({
    mutationFn: async (planId: number) => {
      const response = await createCheckout({ plan_id: planId })
      const checkoutUrl = response.data?.pay_link
      if (!response.success || !checkoutUrl) {
        throw new Error(response.message || 'Payment request failed')
      }
      return checkoutUrl
    },
    onSuccess: (checkoutUrl) => {
      redirectToCheckout(checkoutUrl)
    },
    onError: () => {
      toast.error(t('Payment request failed'))
    },
  })

  return (
    <section id='plans' className='bg-[#101112] text-white'>
      <div className='mx-auto max-w-[1320px] px-6 py-16'>
        <div className='mx-auto max-w-3xl text-center'>
          <h2 className='text-3xl font-semibold tracking-[-0.035em] text-balance sm:text-5xl'>
            {t('Choose the plan that fits your work')}
          </h2>
          <p className='mt-4 text-sm leading-7 text-white/55 sm:text-base'>
            {t(
              'Every plan includes one monthly quota pool, refreshed after each successful monthly renewal.'
            )}
          </p>
          <p className='mt-6 text-xs font-semibold tracking-[0.18em] text-[#ef884c] uppercase'>
            {t('Subscription plans')}
          </p>
        </div>

        {plansQuery.isLoading && (
          <div
            className='mt-14 flex min-h-72 items-center justify-center gap-3 text-sm text-white/55'
            role='status'
          >
            <HugeiconsIcon
              icon={Loading03Icon}
              strokeWidth={2}
              className='size-5 animate-spin'
              aria-hidden='true'
            />
            {t('Loading plans...')}
          </div>
        )}

        {plansQuery.isError && (
          <div className='mt-14 rounded-2xl border border-red-400/25 bg-red-400/10 px-6 py-10 text-center'>
            <p className='text-sm text-red-100'>{t('Unable to load plans')}</p>
            <Button
              variant='outline'
              className='mt-5 border-white/20 bg-transparent text-white hover:bg-white/10'
              onClick={() => void plansQuery.refetch()}
            >
              {t('Try again')}
            </Button>
          </div>
        )}

        {plansQuery.isSuccess && landingPlans.length === 0 && (
          <div className='mt-14 rounded-2xl border border-white/10 bg-white/[0.03] px-6 py-14 text-center text-sm text-white/55'>
            {t('No plans available')}
          </div>
        )}

        {plansQuery.isSuccess && landingPlans.length > 0 && (
          <div
            data-slot='landing-plans-grid'
            className='mt-10 grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3'
          >
            {landingPlans.map((record) => (
              <PlanCard
                key={record.plan.id}
                plan={record.plan}
                quotaPerUnit={quotaPerUnit}
                isAuthenticated={props.isAuthenticated}
                pending={
                  checkoutMutation.isPending &&
                  checkoutMutation.variables === record.plan.id
                }
                onSubscribe={(planId) => checkoutMutation.mutate(planId)}
              />
            ))}
          </div>
        )}
      </div>
    </section>
  )
}
