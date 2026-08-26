import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { getUserQuotaDates } from '@/features/dashboard/api'
import { getPerfMetricsSummary } from '@/features/performance-metrics/api'
import { getSelfSubscriptionFull } from '@/features/subscriptions/api'
import { useAuthStore } from '@/stores/auth-store'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { SummaryCards } from '../summary-cards'

vi.mock('@/features/dashboard/api', () => ({
  getUserQuotaDates: vi.fn(),
}))

vi.mock('@/features/performance-metrics/api', () => ({
  getPerfMetricsSummary: vi.fn(),
}))

vi.mock('@/features/subscriptions/api', () => ({
  getSelfSubscriptionFull: vi.fn(),
}))

function renderSummaryCards() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })

  const result = render(
    <QueryClientProvider client={queryClient}>
      <SummaryCards />
    </QueryClientProvider>
  )

  return { ...result, queryClient }
}

function subscriptionResponse(
  quotas: Array<{ total: number; used: number }>
): Awaited<ReturnType<typeof getSelfSubscriptionFull>> {
  return {
    success: true,
    data: {
      billing_preference: 'subscription_first',
      subscriptions: quotas.map((quota, index) => ({
        subscription: {
          id: index + 1,
          user_id: 1,
          plan_id: index + 1,
          status: 'active',
          start_time: 1,
          end_time: 2,
          amount_total: quota.total,
          amount_used: quota.used,
        },
      })),
      all_subscriptions: [],
      stripe_subscriptions: [],
      stripe_invoices: [],
      billing_debt: 0,
    },
  }
}

beforeEach(() => {
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'overview-user',
    role: 1,
    quota: 2_500_000,
    request_count: 10,
  })
  useSystemConfigStore.getState().setConfig({
    currency: { ...DEFAULT_CURRENCY_CONFIG },
  })

  vi.mocked(getUserQuotaDates).mockResolvedValue({ success: true, data: [] })
  vi.mocked(getPerfMetricsSummary).mockResolvedValue({
    success: true,
    data: { models: [] },
  })
  vi.mocked(getSelfSubscriptionFull).mockResolvedValue(
    subscriptionResponse([
      { total: 1_000_000, used: 200_000 },
      { total: 500_000, used: 300_000 },
    ])
  )
})

afterEach(() => {
  useAuthStore.getState().auth.setUser(null)
})

describe('dashboard summary cards', () => {
  test('shows the combined remaining quota from active subscriptions', async () => {
    const { queryClient } = renderSummaryCards()
    const subscriptionCard = await screen.findByRole('group', {
      name: 'Subscription',
    })

    expect(await within(subscriptionCard).findByText('$2')).toBeInTheDocument()
    expect(
      within(subscriptionCard).getByText('Remaining quota')
    ).toBeInTheDocument()

    queryClient.clear()
  })

  test('shows zero when the user has no active subscription', async () => {
    vi.mocked(getSelfSubscriptionFull).mockResolvedValueOnce(
      subscriptionResponse([])
    )
    const { queryClient } = renderSummaryCards()
    const subscriptionCard = await screen.findByRole('group', {
      name: 'Subscription',
    })

    expect(await within(subscriptionCard).findByText('$0')).toBeInTheDocument()

    queryClient.clear()
  })

  test('shows unavailable state when the subscription request fails', async () => {
    vi.mocked(getSelfSubscriptionFull).mockRejectedValueOnce(
      new Error('subscription request failed')
    )
    const { queryClient } = renderSummaryCards()
    const subscriptionCard = await screen.findByRole('group', {
      name: 'Subscription',
    })

    expect(await within(subscriptionCard).findByText('—')).toBeInTheDocument()

    queryClient.clear()
  })

  test('does not reuse subscription quota after switching users', async () => {
    vi.mocked(getSelfSubscriptionFull)
      .mockReset()
      .mockResolvedValueOnce(
        subscriptionResponse([{ total: 1_500_000, used: 500_000 }])
      )
      .mockResolvedValueOnce(
        subscriptionResponse([{ total: 2_500_000, used: 500_000 }])
      )
    const { queryClient } = renderSummaryCards()

    expect(
      await within(
        await screen.findByRole('group', { name: 'Subscription' })
      ).findByText('$2')
    ).toBeInTheDocument()

    act(() => {
      useAuthStore.getState().auth.setUser({
        id: 2,
        username: 'second-overview-user',
        role: 1,
        quota: 2_500_000,
        request_count: 10,
      })
    })

    expect(
      await within(
        await screen.findByRole('group', { name: 'Subscription' })
      ).findByText('$4')
    ).toBeInTheDocument()
    expect(getSelfSubscriptionFull).toHaveBeenCalledTimes(2)

    queryClient.clear()
  })

  test('keeps all five summary values on one row at extra-large widths', () => {
    const { queryClient } = renderSummaryCards()

    expect(
      screen.getByRole('region', { name: 'Account and traffic summary' })
    ).toHaveClass('xl:grid-cols-5')

    queryClient.clear()
  })
})
