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
  weeklyQuota: number
): PublicPlanRecord {
  return {
    plan: {
      id,
      title,
      subtitle,
      price_amount: price,
      currency: 'CNY',
      duration_unit: 'day',
      duration_value: 28,
      quota_reset_period: 'custom',
      quota_reset_custom_seconds: 604800,
      max_purchase_per_user: 0,
      total_amount: weeklyQuota * 500000,
      stripe_checkout_available: true,
      creem_checkout_available: false,
      waffo_checkout_available: false,
    },
  }
}

const plans = [
  createPlan(1, 'Standard', 'For focused individual development', 399, 110),
  createPlan(
    2,
    'Premium',
    'The first choice for professional developers',
    899,
    260
  ),
  createPlan(
    3,
    'Professional',
    'For intensive development and teams',
    1799,
    530
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
}) {
  useSystemConfigStore.setState((state) => ({
    config: {
      ...state.config,
      currency: {
        ...state.config.currency,
        quotaPerUnit:
          options?.quotaPerUnit ?? DEFAULT_CURRENCY_CONFIG.quotaPerUnit,
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
  test('renders backend plan totals and marks the second plan as recommended', async () => {
    const view = await renderPlans({
      loadPlans: async () => ({ success: true, data: plans }),
    })
    try {
      await waitForPlansQuery(view.queryClient, 'success')

      assert.match(view.container.textContent || '', /¥399/)
      assert.match(view.container.textContent || '', /Weekly quota \$110/)
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
          'All plans renew every 4 weeks and refresh the included credits every 7 days.'
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
      const premiumHeading = [...view.container.querySelectorAll('h3')].find(
        (heading) => heading.textContent === 'Premium'
      )
      const premiumCard = premiumHeading?.closest('article')
      assert.ok(premiumCard)
      assert.match(premiumCard.textContent || '', /Recommended/)
      assert.doesNotMatch(
        premiumCard.previousElementSibling?.textContent || '',
        /Recommended/
      )
    } finally {
      await view.cleanup()
    }
  })

  test('shows each subscription plan discount against four weeks of weekly quota', async () => {
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

      const enterpriseHeading = [...view.container.querySelectorAll('h3')].find(
        (heading) => heading.textContent === 'Enterprise plan'
      )
      const enterpriseCard = enterpriseHeading?.closest('article')
      assert.ok(enterpriseCard)
      assert.doesNotMatch(enterpriseCard.textContent || '', /\/10 price/)
    } finally {
      await view.cleanup()
    }
  })

  test('keeps all four offers in one balanced responsive grid', async () => {
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
      assert.ok(grid.classList.contains('min-[768px]:max-[1180px]:grid-cols-2'))
      assert.ok(grid.classList.contains('min-[1180px]:grid-cols-4'))
      assert.equal(grid.children.length, 4)

      const enterpriseHeading = [...view.container.querySelectorAll('h3')].find(
        (heading) => heading.textContent === 'Enterprise plan'
      )
      const enterpriseCard = enterpriseHeading?.closest('article')
      assert.ok(enterpriseCard)
      assert.equal(enterpriseCard.parentElement, grid)
      assert.doesNotMatch(enterpriseCard.className, /col-span/)
    } finally {
      await view.cleanup()
    }
  })

  test('translates dynamic plan names and subtitles in Chinese', async () => {
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

      assert.match(view.container.textContent || '', /标准/)
      assert.match(view.container.textContent || '', /高级/)
      assert.match(view.container.textContent || '', /专业/)
      assert.match(view.container.textContent || '', /适合专注开发的个人/)
      assert.match(view.container.textContent || '', /专业开发者的首选/)
      assert.match(view.container.textContent || '', /适合高强度开发与团队/)
    } finally {
      await view.cleanup()
    }
  })

  test('hides plans when the configured quota conversion no longer matches the fixed public tiers', async () => {
    const view = await renderPlans({
      quotaPerUnit: 250000,
      loadPlans: async () => ({ success: true, data: plans }),
    })
    try {
      await waitForPlansQuery(view.queryClient, 'success')

      assert.match(view.container.textContent || '', /No plans available/)
      assert.doesNotMatch(view.container.textContent || '', /Weekly quota/)
    } finally {
      await view.cleanup()
    }
  })

  test('renders only CNY four-week plans with seven-day quota refresh', async () => {
    const incompatiblePlans: PublicPlanRecord[] = [
      {
        plan: {
          ...plans[0].plan,
          id: 11,
          title: 'Legacy USD plan',
          currency: 'USD',
        },
      },
      {
        plan: {
          ...plans[0].plan,
          id: 12,
          title: 'Thirty day plan',
          duration_value: 30,
        },
      },
      {
        plan: {
          ...plans[0].plan,
          id: 13,
          title: 'Daily quota plan',
          quota_reset_custom_seconds: 86400,
        },
      },
      createPlan(
        14,
        'Unexpected four-week plan',
        'Should not appear on the public landing page',
        699,
        200
      ),
      plans[2],
      plans[0],
      plans[1],
    ]
    const view = await renderPlans({
      loadPlans: async () => ({ success: true, data: incompatiblePlans }),
    })
    try {
      await waitForPlansQuery(view.queryClient, 'success')

      assert.doesNotMatch(
        view.container.textContent || '',
        /Legacy USD plan|Thirty day plan|Daily quota plan/
      )
      assert.match(view.container.textContent || '', /Standard/)
      assert.match(view.container.textContent || '', /Premium/)
      assert.match(view.container.textContent || '', /Professional/)
      assert.doesNotMatch(
        view.container.textContent || '',
        /Unexpected four-week plan/
      )
      const headings = [...view.container.querySelectorAll('h3')].map(
        (heading) => heading.textContent
      )
      assert.deepEqual(headings.slice(0, 3), [
        'Standard',
        'Premium',
        'Professional',
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
      const enterpriseContact = view.container.querySelector<HTMLAnchorElement>(
        'a[href="mailto:contract@tryvalo.com"]'
      )
      assert.ok(enterpriseContact)
      assert.equal(enterpriseContact.textContent?.trim(), 'Contact sales')
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
