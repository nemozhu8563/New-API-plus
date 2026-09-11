import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { fetchTokenKey, getApiKeys } from '@/features/keys/api'
import { useAuthStore } from '@/stores/auth-store'

import { OverviewDashboard } from '../overview-dashboard'

const copyToClipboard = vi.hoisted(() => vi.fn())

vi.mock('@/features/keys/api', () => ({
  fetchTokenKey: vi.fn(),
  getApiKeys: vi.fn(),
}))

vi.mock('@/hooks/use-copy-to-clipboard', () => ({
  useCopyToClipboard: () => ({ copyToClipboard }),
}))

vi.mock('@/features/dashboard/hooks/use-status-data', () => ({
  useDashboardStatus: () => ({
    serverAddress: 'https://test.tryvalo.com/v1',
    announcements: true,
    faq: false,
    uptimeKuma: false,
  }),
}))

vi.mock('../summary-cards', () => ({
  SummaryCards: () => <section>Account summary</section>,
}))

vi.mock('../usage-trend-panel', () => ({
  UsageTrendPanel: () => <section>Usage trend panel</section>,
}))

vi.mock('../announcements-panel', () => ({
  AnnouncementsPanel: () => <section>System announcements panel</section>,
}))

function renderOverviewDashboard() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const rootRoute = createRootRoute({ component: OverviewDashboard })
  const router = createRouter({
    routeTree: rootRoute,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })

  const result = render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )

  return { ...result, queryClient }
}

beforeEach(() => {
  vi.clearAllMocks()
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'new-overview-user',
    role: 1,
    quota: 0,
    used_quota: 0,
    request_count: 0,
  })
  vi.mocked(getApiKeys).mockResolvedValue({
    success: true,
    data: { items: [], total: 0, page: 1, page_size: 10 },
  })
  copyToClipboard.mockResolvedValue(true)
})

afterEach(() => {
  useAuthStore.getState().auth.setUser(null)
})

describe('overview dashboard', () => {
  test('does not show the getting-started guide for a new user', async () => {
    const { queryClient } = renderOverviewDashboard()

    await waitFor(() => expect(getApiKeys).toHaveBeenCalledTimes(1))
    expect(await screen.findByText('No API key yet')).toBeInTheDocument()
    expect(screen.queryByText('Get started')).not.toBeInTheDocument()
    expect(
      screen.queryByText('Complete your first API request')
    ).not.toBeInTheDocument()

    queryClient.clear()
  })

  test('uses the selected two-thirds trend and one-third announcement layout', async () => {
    const { queryClient } = renderOverviewDashboard()

    const mainInsights = await screen.findByTestId('overview-main-insights')
    expect(mainInsights).toHaveClass('xl:grid-cols-3')
    expect(screen.getByTestId('overview-usage-trend')).toHaveClass(
      'xl:col-span-2'
    )
    expect(screen.getByTestId('overview-announcements')).toHaveClass(
      'xl:col-span-1'
    )

    queryClient.clear()
  })

  test('uses a horizontal connection card with separate quick navigation', async () => {
    const { queryClient } = renderOverviewDashboard()

    expect(
      await screen.findByRole('region', { name: 'API Connection' })
    ).toBeInTheDocument()
    expect(screen.getByTestId('overview-connection-grid')).toHaveClass(
      'xl:grid-cols-[minmax(0,1.2fr)_minmax(0,0.9fr)_minmax(9rem,0.45fr)_auto]'
    )

    const quickNavigation = screen.getByRole('navigation', {
      name: 'Quick actions',
    })
    expect(quickNavigation).toHaveTextContent('API Keys')
    expect(quickNavigation).toHaveTextContent('Usage Logs')
    expect(quickNavigation).toHaveTextContent('Pricing')
    expect(quickNavigation).toHaveTextContent('Wallet')

    queryClient.clear()
  })

  test('fetches the full API key before copying it', async () => {
    const user = userEvent.setup()
    vi.mocked(getApiKeys).mockResolvedValue({
      success: true,
      data: {
        items: [
          {
            id: 7,
            name: 'Dashboard key',
            key: 'masked-key',
            status: 1,
            remain_quota: 0,
            used_quota: 0,
            unlimited_quota: true,
            expired_time: -1,
            created_time: 0,
            accessed_time: 0,
            group: '',
            auto_groups: null,
            cross_group_retry: false,
            model_limits_enabled: false,
            model_limits: '',
            allow_ips: '',
          },
        ],
        total: 1,
        page: 1,
        page_size: 10,
      },
    })
    vi.mocked(fetchTokenKey).mockResolvedValue({
      success: true,
      data: { key: 'real-key-material' },
    })

    const { queryClient } = renderOverviewDashboard()

    await user.click(
      await screen.findByRole('button', { name: 'Copy API key' })
    )

    await waitFor(() => expect(fetchTokenKey).toHaveBeenCalledWith(7))
    expect(copyToClipboard).toHaveBeenCalledWith('sk-real-key-material')

    queryClient.clear()
  })

  test('does not copy the masked key when fetching the full key fails', async () => {
    const user = userEvent.setup()
    vi.mocked(getApiKeys).mockResolvedValue({
      success: true,
      data: {
        items: [
          {
            id: 7,
            name: 'Dashboard key',
            key: 'masked-key',
            status: 1,
            remain_quota: 0,
            used_quota: 0,
            unlimited_quota: true,
            expired_time: -1,
            created_time: 0,
            accessed_time: 0,
            group: '',
            auto_groups: null,
            cross_group_retry: false,
            model_limits_enabled: false,
            model_limits: '',
            allow_ips: '',
          },
        ],
        total: 1,
        page: 1,
        page_size: 10,
      },
    })
    vi.mocked(fetchTokenKey).mockResolvedValue({
      success: false,
      message: 'Unable to load key',
    })

    const { queryClient } = renderOverviewDashboard()

    await user.click(
      await screen.findByRole('button', { name: 'Copy API key' })
    )

    await waitFor(() => expect(fetchTokenKey).toHaveBeenCalledWith(7))
    expect(copyToClipboard).not.toHaveBeenCalled()

    queryClient.clear()
  })

  test('disables key copying while the full key request is pending', async () => {
    const user = userEvent.setup()
    vi.mocked(getApiKeys).mockResolvedValue({
      success: true,
      data: {
        items: [
          {
            id: 7,
            name: 'Dashboard key',
            key: 'masked-key',
            status: 1,
            remain_quota: 0,
            used_quota: 0,
            unlimited_quota: true,
            expired_time: -1,
            created_time: 0,
            accessed_time: 0,
            group: '',
            auto_groups: null,
            cross_group_retry: false,
            model_limits_enabled: false,
            model_limits: '',
            allow_ips: '',
          },
        ],
        total: 1,
        page: 1,
        page_size: 10,
      },
    })
    let resolveFullKey:
      | ((result: { success: boolean; data?: { key: string } }) => void)
      | undefined
    vi.mocked(fetchTokenKey).mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveFullKey = resolve
        })
    )

    const { queryClient } = renderOverviewDashboard()
    const copyButton = await screen.findByRole('button', {
      name: 'Copy API key',
    })

    await user.click(copyButton)

    await waitFor(() => expect(copyButton).toBeDisabled())
    resolveFullKey?.({ success: true, data: { key: 'real-key-material' } })
    await waitFor(() => expect(copyButton).toBeEnabled())

    queryClient.clear()
  })
})
