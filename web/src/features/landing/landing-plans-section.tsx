import {
  ArrowRight01Icon,
  Building03Icon,
  Loading03Icon,
  Tick02Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  getPublicPlans,
  paySubscriptionStripe,
} from '@/features/subscriptions/api'
import { formatSubscriptionPrice } from '@/features/subscriptions/lib'
import type { PublicSubscriptionPlan } from '@/features/subscriptions/types'
import { redirectToHostedCheckout } from '@/features/wallet/lib'
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

interface LandingPlanSpec {
  title: string
  subtitle: string
  priceAmount: number
  weeklyQuota: number
  recommended?: boolean
}

const LANDING_PLAN_SPECS: readonly LandingPlanSpec[] = [
  {
    title: 'Standard',
    subtitle: 'For focused individual development',
    priceAmount: 399,
    weeklyQuota: 110,
  },
  {
    title: 'Premium',
    subtitle: 'The first choice for professional developers',
    priceAmount: 899,
    weeklyQuota: 260,
    recommended: true,
  },
  {
    title: 'Professional',
    subtitle: 'For intensive development and teams',
    priceAmount: 1799,
    weeklyQuota: 530,
  },
]

function isFourWeekWeeklyPlan(plan: PublicSubscriptionPlan): boolean {
  return (
    plan.currency.trim().toUpperCase() === 'CNY' &&
    plan.duration_unit === 'day' &&
    Number(plan.duration_value) === 28 &&
    plan.quota_reset_period === 'custom' &&
    Number(plan.quota_reset_custom_seconds) === 604800
  )
}

function PlanCard(props: {
  plan: PublicSubscriptionPlan
  spec: LandingPlanSpec
  isAuthenticated: boolean
  pending: boolean
  onSubscribe: (planId: number) => void
}) {
  const { t } = useTranslation()
  const quotaLabel = `$${Intl.NumberFormat(undefined).format(
    props.spec.weeklyQuota
  )}`
  const signInHref = `/sign-in?redirect=${encodeURIComponent('/#plans')}`

  return (
    <article
      className={`relative flex min-h-[430px] flex-col rounded-[24px] border bg-[#181d25] p-6 shadow-2xl shadow-black/10 ${
        props.spec.recommended
          ? 'border-[#7d7bff] ring-1 ring-[#7d7bff]'
          : 'border-white/12'
      }`}
    >
      {props.spec.recommended && (
        <span className='absolute -top-3 left-1/2 -translate-x-1/2 rounded-full bg-[#6657ff] px-4 py-1 text-xs font-semibold whitespace-nowrap text-white'>
          {t('Recommended')}
        </span>
      )}

      <div>
        <h3 className='text-xl font-semibold text-white'>
          {t(props.spec.title)}
        </h3>
        <p className='mt-1.5 min-h-6 text-sm text-white/55'>
          {t(props.spec.subtitle)}
        </p>
      </div>

      <div className='mt-6 flex items-end gap-1.5 border-b border-white/12 pb-5'>
        <span className='text-4xl font-bold tracking-[-0.04em] text-white'>
          {formatSubscriptionPrice(
            Number(props.plan.price_amount || 0),
            props.plan.currency
          )}
        </span>
        <span className='pb-1 text-sm text-white/55'>/{t('4 weeks')}</span>
      </div>

      <ul className='mt-5 flex-1 space-y-3 text-sm leading-6 text-white/78'>
        {[
          t('Weekly quota {{quota}}', { quota: quotaLabel }),
          t('Credits refresh every 7 days'),
          t('Renews automatically every 4 weeks'),
        ].map((benefit) => (
          <li key={benefit} className='flex items-start gap-3'>
            <HugeiconsIcon
              icon={Tick02Icon}
              strokeWidth={2}
              className={
                props.spec.recommended
                  ? 'mt-1 size-4 shrink-0 text-[#8584ff]'
                  : 'mt-1 size-4 shrink-0 text-white/55'
              }
              aria-hidden='true'
            />
            <span>{benefit}</span>
          </li>
        ))}
      </ul>

      {props.isAuthenticated ? (
        <Button
          className={`mt-6 h-11 w-full rounded-xl ${
            props.spec.recommended
              ? 'bg-[#5d3dff] text-white hover:bg-[#6c50ff]'
              : 'bg-white/12 text-white hover:bg-white/18'
          }`}
          onClick={() => props.onSubscribe(props.plan.id)}
          disabled={props.pending || !props.plan.stripe_checkout_available}
        >
          {props.pending ? t('Opening checkout...') : t('Subscribe now')}
        </Button>
      ) : (
        <Button
          className={`mt-6 h-11 w-full rounded-xl ${
            props.spec.recommended
              ? 'bg-[#5d3dff] text-white hover:bg-[#6c50ff]'
              : 'bg-white/12 text-white hover:bg-white/18'
          }`}
          render={<a href={signInHref} />}
        >
          {t('Sign in to subscribe')}
        </Button>
      )}
    </article>
  )
}

function EnterpriseCard() {
  const { t } = useTranslation()

  return (
    <article className='relative flex min-h-[430px] flex-col overflow-hidden rounded-[24px] border border-[#ef884c]/55 bg-[#211b1d] p-6 shadow-2xl shadow-[#ef884c]/10'>
      <div className='flex size-12 items-center justify-center rounded-2xl bg-[#ef884c] text-white shadow-lg shadow-[#ef884c]/20'>
        <HugeiconsIcon
          icon={Building03Icon}
          strokeWidth={1.8}
          className='size-6'
          aria-hidden='true'
        />
      </div>
      <span className='absolute top-6 right-6 rounded-full border border-[#ef884c]/45 bg-[#ef884c]/10 px-3 py-1 text-xs font-medium text-[#ffb17f]'>
        {t('Enterprise')}
      </span>

      <h3 className='mt-6 text-xl font-semibold text-white'>
        {t('Enterprise plan')}
      </h3>
      <p className='mt-1.5 text-sm leading-6 text-white/55'>
        {t('Tailored quotas and concurrency for your team.')}
      </p>

      <ul className='mt-5 flex-1 space-y-3 text-sm leading-6 text-white/78'>
        {[
          'Custom weekly quota and concurrency',
          'Dedicated API access and usage management',
          'Priority support and incident handling',
          'Contracts, invoices, and company billing',
        ].map((benefit) => (
          <li key={benefit} className='flex items-start gap-3'>
            <HugeiconsIcon
              icon={Tick02Icon}
              strokeWidth={2}
              className='mt-1 size-4 shrink-0 text-[#ef884c]'
              aria-hidden='true'
            />
            <span>{t(benefit)}</span>
          </li>
        ))}
      </ul>

      <Button
        className='mt-6 h-11 w-full rounded-xl bg-[#ef884c] text-white hover:bg-[#f39a63]'
        render={<a href='mailto:contract@tryvalo.com' />}
      >
        {t('Contact sales')}
        <HugeiconsIcon
          icon={ArrowRight01Icon}
          strokeWidth={2}
          data-icon='inline-end'
          aria-hidden='true'
        />
      </Button>
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
  const landingPlans = useMemo(() => {
    const publicPlans = plansQuery.data || []
    return LANDING_PLAN_SPECS.flatMap((spec) => {
      const expectedQuota = spec.weeklyQuota * quotaPerUnit
      const record = publicPlans.find(
        ({ plan }) =>
          isFourWeekWeeklyPlan(plan) &&
          plan.stripe_checkout_available &&
          Math.abs(Number(plan.price_amount) - spec.priceAmount) < 0.001 &&
          Math.abs(Number(plan.total_amount) - expectedQuota) < 0.5
      )
      return record ? [{ plan: record.plan, spec }] : []
    })
  }, [plansQuery.data, quotaPerUnit])
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
              'All plans renew every 4 weeks and refresh the included credits every 7 days.'
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

        {plansQuery.isSuccess && (
          <div className='mt-10 grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
            {landingPlans.map((record) => (
              <PlanCard
                key={record.plan.id}
                plan={record.plan}
                spec={record.spec}
                isAuthenticated={props.isAuthenticated}
                pending={
                  checkoutMutation.isPending &&
                  checkoutMutation.variables === record.plan.id
                }
                onSubscribe={(planId) => checkoutMutation.mutate(planId)}
              />
            ))}
            <EnterpriseCard />
          </div>
        )}
      </div>
    </section>
  )
}
