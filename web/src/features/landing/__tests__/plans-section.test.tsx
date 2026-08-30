import assert from 'node:assert/strict'

import { Window } from 'happy-dom'
import { afterAll as after, describe, test } from 'vitest'

import type { PublicPlanRecord } from '@/features/subscriptions/types'

const domWindow = new Window({ url: 'https://test.tryvalo.com/' })
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLAnchorElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'MouseEvent',
  'CustomEvent',
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

const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { DEFAULT_CURRENCY_CONFIG, useSystemConfigStore } =
  await import('@/stores/system-config-store')
const { LandingPlansSection } = await import('../landing-plans-section')

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function createPlan(
  id: number,
  title: string,
  subtitle: string,
  price: number,
  monthlyQuota: number,
  recommended = false,
  stripeCheckoutAvailable = true
): PublicPlanRecord {
  return {
    plan: {
      id,
      title,
      subtitle,
      recommended,
      price_amount: price,
      currency: 'CNY',
      duration_unit: 'month',
      duration_value: 1,
      quota_reset_period: 'billing_cycle',
      quota_reset_custom_seconds: 0,
      max_purchase_per_user: 0,
      total_amount: monthlyQuota * 500000,
      stripe_checkout_available: stripeCheckoutAvailable,
      creem_checkout_available: false,
      waffo_checkout_available: false,
    },
  }
}

const plans = [
  createPlan(1, 'Standard', 'For focused individual development', 399, 440),
  createPlan(
    2,
    'Premium',
    'The first choice for professional developers',
    899,
    1040
  ),
  createPlan(
    3,
    'Professional',
    'For intensive development and teams',
    1799,
    2120,
    true
  ),
]

async function renderPlans(options?: {
  isAuthenticated?: boolean
  language?: string
  translations?: Record<string, string>
  loadPlans?: () => Promise<{
    success: boolean
    message?: string
    data?: PublicPlanRecord[]
  }>
  createCheckout?: (request: { plan_id: number }) => Promise<{
    success: boolean
    message?: string
    data?: { pay_link?: string }
  }>
  redirectToCheckout?: (checkoutUrl: string) => void
  quotaPerUnit?: number
  quotaDisplayType?: 'USD' | 'CNY' | 'TOKENS' | 'CUSTOM'
  usdExchangeRate?: number
}) {
  useSystemConfigStore.setState((state) => ({
    config: {
      ...state.config,
      currency: {
        ...state.config.currency,
        quotaPerUnit:
          options?.quotaPerUnit ?? DEFAULT_CURRENCY_CONFIG.quotaPerUnit,
        quotaDisplayType:
          options?.quotaDisplayType ?? DEFAULT_CURRENCY_CONFIG.quotaDisplayType,
        usdExchangeRate:
          options?.usdExchangeRate ?? DEFAULT_CURRENCY_CONFIG.usdExchangeRate,
      },
    },
  }))
  const i18n = createInstance()
  await i18n.use(initReactI18next).init({
    lng: options?.language ?? 'en',
    fallbackLng: 'en',
    resources: {
      [options?.language ?? 'en']: {
        translation: options?.translations ?? {},
      },
    },
  })
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={queryClient}>
          <LandingPlansSection
            isAuthenticated={options?.isAuthenticated ?? false}
            loadPlans={options?.loadPlans}
            createCheckout={options?.createCheckout}
            redirectToCheckout={options?.redirectToCheckout ?? (() => {})}
          />
        </QueryClientProvider>
      </I18nextProvider>
    )
  })
  return {
    container,
    queryClient,
    root,
    async cleanup() {
      await queryClient.cancelQueries()
      await act(async () => {
        root.unmount()
        await Promise.resolve()
      })
      queryClient.clear()
      container.remove()
      useSystemConfigStore.setState((state) => ({
        config: {
          ...state.config,
          currency: {
            ...state.config.currency,
            quotaPerUnit: DEFAULT_CURRENCY_CONFIG.quotaPerUnit,
            quotaDisplayType: DEFAULT_CURRENCY_CONFIG.quotaDisplayType,
            usdExchangeRate: DEFAULT_CURRENCY_CONFIG.usdExchangeRate,
          },
        },
      }))
    },
  }
}

async function waitForPlansQuery(
  queryClient: InstanceType<typeof QueryClient>,
  expectedStatus: 'success' | 'error'
) {
  const queryKey = ['public-subscription-plans']
  await act(async () => {
    const currentQuery = queryClient.getQueryCache().find({ queryKey })
    if (currentQuery?.state.status !== expectedStatus) {
      await new Promise<void>((resolve, reject) => {
        const timeout = setTimeout(() => {
          unsubscribe()
          reject(new Error(`Plans query did not reach ${expectedStatus}`))
        }, 1_000)
        const unsubscribe = queryClient.getQueryCache().subscribe(() => {
          const query = queryClient.getQueryCache().find({ queryKey })
          if (query?.state.status !== expectedStatus) return
          clearTimeout(timeout)
          unsubscribe()
          resolve()
        })
      })
    }
    await new Promise<void>((resolve) => setTimeout(resolve, 0))
  })
}

after(() => {
  domWindow.close()
})

describe('landing subscription plans', { concurrent: false }, () => {
  test('renders backend monthly quotas and uses the configured recommendation', async () => {
    const view = await renderPlans({
      loadPlans: async () => ({ success: true, data: plans }),
    })
    try {
      await waitForPlansQuery(view.queryClient, 'success')

      assert.match(view.container.textContent || '', /¥399/)
      assert.match(view.container.textContent || '', /Monthly quota \$440/)
      assert.doesNotMatch(
        view.container.textContent || '',
        /per[- ]?(?:token|model|request)|unit price/i
      )
      const sectionHeading = [...view.container.querySelectorAll('h2')].find(
        (heading) =>
          heading.textContent === 'Choose the plan that fits your work'
      )
      const sectionSubtitle = [...view.container.querySelectorAll('p')].find(
        (paragraph) =>
          paragraph.textContent ===
          'Every plan includes one monthly quota pool, refreshed after each successful monthly renewal.'
      )
      const sectionLabel = [...view.container.querySelectorAll('p')].find(
        (paragraph) => paragraph.textContent === 'Subscription plans'
      )
      assert.ok(sectionHeading)
      assert.ok(sectionSubtitle)
      assert.ok(sectionLabel)
      assert.ok(
        sectionHeading.compareDocumentPosition(sectionSubtitle) &
          Node.DOCUMENT_POSITION_FOLLOWING
      )
      assert.ok(
        sectionSubtitle.compareDocumentPosition(sectionLabel) &
          Node.DOCUMENT_POSITION_FOLLOWING
      )
      const professionalHeading = [
        ...view.container.querySelectorAll('h3'),
      ].find((heading) => heading.textContent === 'Professional')
      const professionalCard = professionalHeading?.closest('article')
      assert.ok(professionalCard)
      assert.match(professionalCard.textContent || '', /Recommended/)
      const premiumHeading = [...view.container.querySelectorAll('h3')].find(
        (heading) => heading.textContent === 'Premium'
      )
      const premiumCard = premiumHeading?.closest('article')
      assert.ok(premiumCard)
      assert.doesNotMatch(premiumCard.textContent || '', /Recommended/)
    } finally {
      await view.cleanup()
    }
  })

  test('calculates each discount from the configured monthly quota', async () => {
    const view = await renderPlans({
      loadPlans: async () => ({ success: true, data: plans }),
    })
    try {
      await waitForPlansQuery(view.queryClient, 'success')

      for (const [title, discount] of [
        ['Standard', '9/10 price'],
        ['Premium', '8.6/10 price'],
        ['Professional', '8.4/10 price'],
      ]) {
        const heading = [...view.container.querySelectorAll('h3')].find(
          (candidate) => candidate.textContent === title
        )
        const card = heading?.closest('article')
        assert.ok(card)
        assert.ok(card.textContent?.includes(discount))
      }
    } finally {
      await view.cleanup()
    }
  })

  test('renders every configured plan in one responsive grid', async () => {
    const view = await renderPlans({
      loadPlans: async () => ({ success: true, data: plans }),
    })
    try {
      await waitForPlansQuery(view.queryClient, 'success')

      const grid = view.container.querySelector(
        '[data-slot="landing-plans-grid"]'
      )
      assert.ok(grid)
      assert.ok(grid.classList.contains('grid-cols-1'))
      assert.ok(grid.classList.contains('md:grid-cols-2'))
      assert.ok(grid.classList.contains('xl:grid-cols-3'))
      assert.equal(grid.children.length, plans.length)
    } finally {
      await view.cleanup()
    }
  })

  test('renders backend plan names literally instead of treating them as translation keys', async () => {
    const view = await renderPlans({
      language: 'zh',
      translations: {
        Standard: '标准',
        Premium: '高级',
        Professional: '专业',
        'For focused individual development': '适合专注开发的个人',
        'The first choice for professional developers': '专业开发者的首选',
        'For intensive development and teams': '适合高强度开发与团队',
      },
      loadPlans: async () => ({ success: true, data: plans }),
    })
    try {
      await waitForPlansQuery(view.queryClient, 'success')

      assert.match(view.container.textContent || '', /Standard/)
      assert.match(view.container.textContent || '', /Premium/)
      assert.match(view.container.textContent || '', /Professional/)
      assert.match(
        view.container.textContent || '',
        /For focused individual development/
      )
      assert.doesNotMatch(view.container.textContent || '', /标准|高级/)
    } finally {
      await view.cleanup()
    }
  })

  test('uses the configured quota conversion without hiding backend plans', async () => {
    const view = await renderPlans({
      quotaPerUnit: 250000,
      loadPlans: async () => ({ success: true, data: plans }),
    })
    try {
      await waitForPlansQuery(view.queryClient, 'success')

      assert.match(view.container.textContent || '', /Standard/)
      assert.match(view.container.textContent || '', /Monthly quota \$880/)
    } finally {
      await view.cleanup()
    }
  })

  test('keeps the public monthly allowance in USD for CNY display settings', async () => {
    const view = await renderPlans({
      quotaDisplayType: 'CNY',
      usdExchangeRate: 7.3,
      loadPlans: async () => ({ success: true, data: plans }),
    })
    try {
      await waitForPlansQuery(view.queryClient, 'success')

      assert.match(view.container.textContent || '', /Monthly quota \$440/)
      assert.doesNotMatch(
        view.container.textContent || '',
        /Monthly quota.*3,212/
      )
    } finally {
      await view.cleanup()
    }
  })

  test('localizes legacy credit copy as a USD billing-cycle allowance', async () => {
    const legacyPlan = createPlan(
      1,
      'Standard',
      'Includes 290 Credits per billing cycle',
      259,
      290
    )
    const view = await renderPlans({
      language: 'zh',
      quotaDisplayType: 'TOKENS',
      translations: {
        'Includes {{amount}} per billing cycle': '每个账期包含 {{amount}}',
      },
      loadPlans: async () => ({ success: true, data: [legacyPlan] }),
    })
    try {
      await waitForPlansQuery(view.queryClient, 'success')

      assert.match(view.container.textContent || '', /每个账期包含 \$290/)
      assert.doesNotMatch(view.container.textContent || '', /Credits|145M/)
    } finally {
      await view.cleanup()
    }
  })

  test('renders every public plan in the API order without fixed tier names', async () => {
    const configuredPlans: PublicPlanRecord[] = [
      createPlan(14, 'Team', 'Shared quota for a small team', 1299, 1500),
      plans[2],
      createPlan(15, 'Starter', 'A lower-cost entry plan', 99, 120),
      plans[0],
    ]
    const view = await renderPlans({
      loadPlans: async () => ({ success: true, data: configuredPlans }),
    })
    try {
      await waitForPlansQuery(view.queryClient, 'success')

      assert.match(view.container.textContent || '', /Team/)
      assert.match(view.container.textContent || '', /Starter/)
      assert.match(view.container.textContent || '', /Standard/)
      assert.match(view.container.textContent || '', /Professional/)
      const headings = [...view.container.querySelectorAll('h3')].map(
        (heading) => heading.textContent
      )
      assert.deepEqual(headings, [
        'Team',
        'Professional',
        'Starter',
        'Standard',
      ])
    } finally {
      await view.cleanup()
    }
  })

  test('sends anonymous subscribers to sign in and returns them to plans', async () => {
    const view = await renderPlans({
      loadPlans: async () => ({ success: true, data: plans }),
    })
    try {
      await waitForPlansQuery(view.queryClient, 'success')

      const signInLink = view.container.querySelector<HTMLAnchorElement>(
        'a[href="/sign-in?redirect=%2F%23plans"]'
      )
      assert.ok(signInLink)
      assert.equal(signInLink.textContent?.trim(), 'Sign in to subscribe')
    } finally {
      await view.cleanup()
    }
  })

  test('posts the selected plan when an authenticated user subscribes', async () => {
    let requestedPlanId = 0
    let redirectedCheckoutUrl = ''
    let resolveCheckout: (() => void) | undefined
    const checkoutCompleted = new Promise<void>((resolve) => {
      resolveCheckout = resolve
    })
    const view = await renderPlans({
      isAuthenticated: true,
      loadPlans: async () => ({ success: true, data: plans }),
      createCheckout: async (request) => {
        requestedPlanId = request.plan_id
        return {
          success: true,
          data: { pay_link: 'https://checkout.stripe.test/session' },
        }
      },
      redirectToCheckout: (checkoutUrl) => {
        redirectedCheckoutUrl = checkoutUrl
        resolveCheckout?.()
      },
    })
    try {
      await waitForPlansQuery(view.queryClient, 'success')
      const standardHeading = [...view.container.querySelectorAll('h3')].find(
        (heading) => heading.textContent === 'Standard'
      )
      const subscribeButton = standardHeading
        ?.closest('article')
        ?.querySelector<HTMLButtonElement>('button')
      assert.ok(subscribeButton)

      await act(async () => {
        subscribeButton.dispatchEvent(
          new MouseEvent('click', { bubbles: true })
        )
        await checkoutCompleted
      })

      assert.equal(requestedPlanId, 1)
      assert.equal(
        redirectedCheckoutUrl,
        'https://checkout.stripe.test/session'
      )
    } finally {
      await view.cleanup()
    }
  })

  test('keeps a public plan visible when Stripe checkout is not configured', async () => {
    let checkoutRequested = false
    const unavailablePlan = createPlan(
      9,
      'Coming soon',
      'Visible before checkout is enabled',
      199,
      220,
      false,
      false
    )
    const view = await renderPlans({
      isAuthenticated: true,
      loadPlans: async () => ({ success: true, data: [unavailablePlan] }),
      createCheckout: async () => {
        checkoutRequested = true
        return { success: false }
      },
    })
    try {
      await waitForPlansQuery(view.queryClient, 'success')

      assert.match(view.container.textContent || '', /Coming soon/)
      const button = view.container.querySelector<HTMLButtonElement>('button')
      assert.ok(button)
      assert.equal(button.textContent?.trim(), 'Not available')
      assert.equal(button.disabled, true)
      assert.equal(checkoutRequested, false)
    } finally {
      await view.cleanup()
    }
  })

  test('shows a loading state while plans are pending', async () => {
    let resolveRequest: (() => void) | undefined
    const pendingRequest = new Promise<void>((resolve) => {
      resolveRequest = resolve
    })
    const view = await renderPlans({
      loadPlans: async () => {
        await pendingRequest
        return {
          success: true,
          data: plans,
        }
      },
    })
    try {
      assert.ok(view.container.querySelector('[role="status"]'))
      assert.match(view.container.textContent || '', /Loading plans/)

      resolveRequest?.()
      await waitForPlansQuery(view.queryClient, 'success')
    } finally {
      await view.cleanup()
    }
  })

  test('shows a stable empty state when no plans are enabled', async () => {
    const view = await renderPlans({
      loadPlans: async () => ({ success: true, data: [] }),
    })
    try {
      await waitForPlansQuery(view.queryClient, 'success')
      assert.match(view.container.textContent || '', /No plans available/)
      assert.equal(
        view.container.querySelector('[data-slot="landing-plans-grid"]'),
        null
      )
    } finally {
      await view.cleanup()
    }
  })

  test('shows a retry action when plan loading fails', async () => {
    const view = await renderPlans({
      loadPlans: async () => {
        throw new Error('network unavailable')
      },
    })
    try {
      await waitForPlansQuery(view.queryClient, 'error')
      assert.match(view.container.textContent || '', /Unable to load plans/)
      assert.ok(
        [...view.container.querySelectorAll('button')].some(
          (button) => button.textContent?.trim() === 'Try again'
        )
      )
    } finally {
      await view.cleanup()
    }
  })
})
