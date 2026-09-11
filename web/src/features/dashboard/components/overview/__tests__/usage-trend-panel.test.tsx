import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18n from 'i18next'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { getUserQuotaDates } from '@/features/dashboard/api'
import { useAuthStore } from '@/stores/auth-store'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { UsageTrendPanel } from '../usage-trend-panel'

vi.mock('@/features/dashboard/api', () => ({
  getUserQuotaDates: vi.fn(),
}))

function renderUsageTrendPanel() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })

  const result = render(
    <QueryClientProvider client={queryClient}>
      <UsageTrendPanel />
    </QueryClientProvider>
  )

  return { ...result, queryClient }
}

beforeEach(() => {
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'usage-trend-user',
    role: 1,
    quota: 2_500_000,
    request_count: 10,
  })
  useSystemConfigStore.getState().setConfig({
    currency: { ...DEFAULT_CURRENCY_CONFIG },
  })
})

afterEach(() => {
  useAuthStore.getState().auth.setUser(null)
})

describe('overview usage trend', () => {
  test('aggregates hourly usage into seven accessible daily points', async () => {
    const now = Math.floor(Date.now() / 1000)
    vi.mocked(getUserQuotaDates).mockResolvedValueOnce({
      success: true,
      data: [
        { created_at: now - 3600, count: 2, quota: 500_000 },
        { created_at: now - 1800, count: 3, quota: 1_000_000 },
        { created_at: now - 86_400, count: 4, quota: 250_000 },
      ],
    })

    const { queryClient } = renderUsageTrendPanel()

    const requests = await screen.findByRole('group', { name: 'Requests' })
    expect(within(requests).getByText('9')).toBeInTheDocument()

    const cost = screen.getByRole('group', { name: 'Cost' })
    expect(within(cost).getByText('$3.5')).toBeInTheDocument()

    const table = screen.getByRole('table', { name: 'Usage trend' })
    expect(within(table).getAllByRole('row')).toHaveLength(8)

    queryClient.clear()
  })

  test('loads a bounded thirty-day range when the user changes the trend period', async () => {
    vi.mocked(getUserQuotaDates).mockResolvedValue({
      success: true,
      data: [],
    })
    const user = userEvent.setup()
    const { queryClient } = renderUsageTrendPanel()

    await waitFor(() => expect(getUserQuotaDates).toHaveBeenCalledTimes(1))
    await user.click(screen.getByRole('button', { name: '30 Days' }))

    await waitFor(() => expect(getUserQuotaDates).toHaveBeenCalledTimes(2))
    expect(screen.getByRole('button', { name: '30 Days' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )

    const latestParams = vi.mocked(getUserQuotaDates).mock.calls.at(-1)?.[0]
    expect(latestParams).toBeDefined()
    expect(
      Number(latestParams?.end_timestamp) -
        Number(latestParams?.start_timestamp)
    ).toBeLessThanOrEqual(30 * 24 * 60 * 60)
    expect(
      Number(latestParams?.end_timestamp) -
        Number(latestParams?.start_timestamp)
    ).toBeGreaterThan(29 * 24 * 60 * 60)

    queryClient.clear()
  })

  test('shows a clear fallback when usage data cannot be loaded', async () => {
    vi.mocked(getUserQuotaDates).mockRejectedValueOnce(
      new Error('usage request failed')
    )
    const { queryClient } = renderUsageTrendPanel()

    expect(
      await screen.findByText('Failed to load usage trend')
    ).toBeInTheDocument()

    queryClient.clear()
  })

  test('renders with the zhCN interface locale', async () => {
    vi.mocked(getUserQuotaDates).mockResolvedValueOnce({
      success: true,
      data: [],
    })
    await i18n.changeLanguage('zhCN')

    try {
      const { queryClient } = renderUsageTrendPanel()

      expect(await screen.findByText('Usage trend')).toBeInTheDocument()
      expect(
        await screen.findByRole('table', { name: 'Usage trend' })
      ).toBeInTheDocument()

      queryClient.clear()
    } finally {
      await i18n.changeLanguage('en')
    }
  })
})
