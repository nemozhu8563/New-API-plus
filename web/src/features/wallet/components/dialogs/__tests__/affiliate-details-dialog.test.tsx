import assert from 'node:assert/strict'

import { Window } from 'happy-dom'
import { afterAll as after, afterEach, describe, test } from 'vitest'

import type { AffiliateSummary } from '@/features/affiliates'
import { api } from '@/lib/api'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MouseEvent',
  'KeyboardEvent',
  'FocusEvent',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { AffiliateDetailsDialog } = await import('../affiliate-details-dialog')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Affiliate details': 'Affiliate details',
        Conversions: 'Conversions',
        'Failed to load affiliate records': 'Failed to load affiliate records',
        'Invited accounts': 'Invited accounts',
        'Next page': 'Next page',
        'No affiliate records found': 'No affiliate records found',
        of: 'of',
        'Previous page': 'Previous page',
        Redemptions: 'Redemptions',
        'Reward records': 'Reward records',
        Showing: 'Showing',
        'View invited accounts, redeemed face value, rewards, conversions, and withdrawals.':
          'View invited accounts, redeemed face value, rewards, conversions, and withdrawals.',
        Withdrawals: 'Withdrawals',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const nonAgentSummary: AffiliateSummary = {
  is_agent: false,
  enabled: false,
  commission_rate_bps: 0,
  cash_withdrawal_enabled: false,
  available_quota: 0,
  pending_withdrawal_quota: 0,
  converted_quota: 0,
  withdrawn_quota: 0,
  total_commission_quota: 0,
  invitee_count: 0,
  ordinary_reward_quota: 0,
  total_reward_quota: 0,
  redemption_count: 0,
  redeemed_quota: 0,
}

const originalGet = api.get
let requestedUrls: string[] = []

function installDetailsApiRecorder(total = 0) {
  api.get = (async (url: string) => {
    requestedUrls.push(url)
    const params = new URL(url, 'https://example.com').searchParams
    return {
      data: {
        success: true,
        data: {
          page: Number(params.get('p') ?? 1),
          page_size: Number(params.get('page_size') ?? 10),
          total,
          items: [],
        },
      },
    }
  }) as unknown as typeof api.get
}

async function flushQueries() {
  await act(async () => {
    await Promise.resolve()
    await Promise.resolve()
    await new Promise<void>((resolve) => domWindow.setTimeout(resolve, 0))
  })
}

async function waitForBodyText(text: string) {
  await act(async () => {
    if (document.body.textContent?.includes(text)) {
      return
    }

    await new Promise<void>((resolve, reject) => {
      let settled = false
      const finish = () => {
        if (settled) {
          return
        }
        settled = true
        domWindow.clearTimeout(timeout)
        observer.disconnect()
        resolve()
      }
      const observer = new MutationObserver(() => {
        if (document.body.textContent?.includes(text)) {
          finish()
        }
      })
      const timeout = domWindow.setTimeout(() => {
        settled = true
        observer.disconnect()
        reject(new Error(`Timed out waiting for body text: ${text}`))
      }, 1000)
      observer.observe(document.body, {
        childList: true,
        characterData: true,
        subtree: true,
      })
      if (document.body.textContent?.includes(text)) {
        finish()
      }
    })
  })
}

async function renderDetailsDialog(
  open: boolean,
  summary: AffiliateSummary | null
) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: 10_000 },
    },
  })

  const render = async (nextOpen: boolean) => {
    await act(async () => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <I18nextProvider i18n={i18n}>
            <AffiliateDetailsDialog
              open={nextOpen}
              onOpenChange={() => undefined}
              summary={summary}
            />
          </I18nextProvider>
        </QueryClientProvider>
      )
    })
    await flushQueries()
  }

  await render(open)
  return { container, queryClient, render, root }
}

async function cleanupDetailsDialog(
  result: Awaited<ReturnType<typeof renderDetailsDialog>>
) {
  await act(async () => result.root.unmount())
  result.container.remove()
  result.queryClient.clear()
}

afterEach(() => {
  api.get = originalGet
  requestedUrls = []
})

describe('affiliate details dialog', () => {
  after(() => {
    domWindow.close()
  })

  test('does not load records while closed and hides agent-only tabs for ordinary inviters', async () => {
    installDetailsApiRecorder()
    const result = await renderDetailsDialog(false, nonAgentSummary)
    assert.deepEqual(requestedUrls, [])

    await result.render(true)
    assert.deepEqual(requestedUrls, [
      '/api/user/affiliate/invitees?p=1&page_size=10',
    ])
    assert.equal(document.body.textContent?.includes('Invited accounts'), true)
    assert.equal(document.body.textContent?.includes('Redemptions'), true)
    assert.equal(document.body.textContent?.includes('Reward records'), true)
    assert.equal(document.body.textContent?.includes('Conversions'), false)
    assert.equal(document.body.textContent?.includes('Withdrawals'), false)

    await cleanupDetailsDialog(result)
  })

  test('reloads records when reopened inside the global cache window', async () => {
    installDetailsApiRecorder()
    const result = await renderDetailsDialog(true, nonAgentSummary)
    assert.equal(requestedUrls.length, 1)

    await result.render(false)
    await result.render(true)

    assert.deepEqual(requestedUrls, [
      '/api/user/affiliate/invitees?p=1&page_size=10',
      '/api/user/affiliate/invitees?p=1&page_size=10',
    ])

    await cleanupDetailsDialog(result)
  })

  test('switches record endpoints and advances pagination without reusing stale pages', async () => {
    installDetailsApiRecorder(15)
    const result = await renderDetailsDialog(true, {
      ...nonAgentSummary,
      is_agent: true,
      enabled: true,
      commission_rate_bps: 800,
    })
    assert.deepEqual(requestedUrls, [
      '/api/user/affiliate/invitees?p=1&page_size=10',
    ])

    const redemptionsTab = [...document.querySelectorAll('button')].find(
      (button) => button.textContent === 'Redemptions'
    )
    assert.ok(redemptionsTab)
    await act(async () => redemptionsTab.click())
    await flushQueries()
    assert.equal(
      requestedUrls.at(-1),
      '/api/user/affiliate/redemptions?p=1&page_size=10'
    )

    const nextButton = document.querySelector<HTMLButtonElement>(
      'button[aria-label="Next page"]'
    )
    assert.ok(nextButton)
    assert.equal(nextButton.disabled, false)
    await act(async () => nextButton.click())
    await flushQueries()
    assert.equal(
      requestedUrls.at(-1),
      '/api/user/affiliate/redemptions?p=2&page_size=10'
    )
    await waitForBodyText('2/2')
    assert.equal(document.body.textContent?.includes('2/2'), true)

    await cleanupDetailsDialog(result)
  })
})
