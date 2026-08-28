import assert from 'node:assert/strict'

import { Window } from 'happy-dom'
import { afterAll as after, afterEach, describe, test } from 'vitest'

const domWindow = new Window({ url: 'https://test.tryvalo.com/wallet' })
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
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { api } = await import('@/lib/api')
const { SubscriptionPlansCard } = await import('../subscription-plans-card')

const originalAdapter = api.defaults.adapter
const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const availablePlan = {
  plan: {
    id: 1,
    title: 'Standard',
    price_amount: 399,
    currency: 'CNY',
    duration_unit: 'month',
    duration_value: 1,
    quota_reset_period: 'billing_cycle',
    quota_reset_custom_seconds: 0,
    recommended: true,
    max_purchase_per_user: 0,
    total_amount: 55_000_000,
    stripe_checkout_available: false,
    creem_checkout_available: false,
    waffo_checkout_available: false,
  },
}

const stripeTopupInfo = {
  enable_online_topup: false,
  enable_stripe_topup: true,
  enable_stripe_subscription: true,
  pay_methods: [],
  min_topup: 1,
  stripe_min_topup: 20,
  amount_options: [],
  discount: {},
}

const activeSubscription = {
  subscription: {
    id: 1,
    user_id: 1,
    plan_id: 1,
    status: 'active',
    start_time: 1_700_000_000,
    end_time: 1_900_000_000,
    amount_total: 55_000_000,
    amount_used: 0,
  },
}

afterEach(() => {
  api.defaults.adapter = originalAdapter
  document.body.replaceChildren()
})

after(() => {
  domWindow.close()
})

describe('subscription plans visibility', () => {
  test('hides billing status and disables a plan without a configured checkout', async () => {
    let completedRequests = 0
    let resolveRequests: (() => void) | undefined
    const requestsComplete = new Promise<void>((resolve) => {
      resolveRequests = resolve
    })

    api.defaults.adapter = async (config) => {
      completedRequests += 1
      if (completedRequests === 2) resolveRequests?.()

      const data =
        config.url === '/api/subscription/plans'
          ? { success: true, data: [availablePlan] }
          : {
              success: true,
              data: {
                billing_preference: 'subscription_first',
                subscriptions: [],
                all_subscriptions: [],
                stripe_subscriptions: [
                  {
                    subscription_id: 'sub_stale',
                    customer_id: 'cus_1',
                    plan_id: 1,
                    plan_title: 'Standard',
                    status: 'unknown',
                    cancel_at_period_end: false,
                    cancel_at: 0,
                    current_period_end: 0,
                    livemode: false,
                  },
                ],
                stripe_invoices: [
                  {
                    invoice_id: 'in_stale',
                    subscription_id: 'sub_stale',
                    plan_title: 'Standard',
                    amount_paid_minor: 39_900,
                    currency: 'cny',
                    period_start: 1_700_000_000,
                    period_end: 1_702_419_200,
                    created_at: 1_700_000_000,
                    livemode: false,
                  },
                ],
                billing_debt: 100,
              },
            }

      return {
        data,
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }

    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <SubscriptionPlansCard topupInfo={stripeTopupInfo} />
        </I18nextProvider>
      )
    })
    await act(async () => {
      await requestsComplete
      await Promise.resolve()
    })

    const text = container.textContent || ''
    assert.match(text, /Standard/)
    assert.doesNotMatch(text, /My Subscriptions/)
    assert.doesNotMatch(text, /Stripe billing/)
    assert.doesNotMatch(text, /Billing history/)
    const unavailableButton = [...container.querySelectorAll('button')].find(
      (button) => button.textContent?.includes('Not available')
    )
    assert.ok(unavailableButton)
    assert.equal(unavailableButton.disabled, true)

    await act(async () => root.unmount())
    container.remove()
  })

  test('keeps subscription and Stripe billing details for a subscribed user', async () => {
    let completedRequests = 0
    let resolveRequests: (() => void) | undefined
    const requestsComplete = new Promise<void>((resolve) => {
      resolveRequests = resolve
    })

    api.defaults.adapter = async (config) => {
      completedRequests += 1
      if (completedRequests === 2) resolveRequests?.()

      const data =
        config.url === '/api/subscription/plans'
          ? { success: true, data: [availablePlan] }
          : {
              success: true,
              data: {
                billing_preference: 'subscription_first',
                subscriptions: [activeSubscription],
                all_subscriptions: [activeSubscription],
                stripe_subscriptions: [
                  {
                    subscription_id: 'sub_active',
                    customer_id: 'cus_1',
                    plan_id: 1,
                    plan_title: 'Standard',
                    status: 'active',
                    cancel_at_period_end: false,
                    cancel_at: 0,
                    current_period_end: 1_900_000_000,
                    livemode: false,
                  },
                ],
                stripe_invoices: [
                  {
                    invoice_id: 'in_paid',
                    subscription_id: 'sub_active',
                    plan_title: 'Standard',
                    amount_paid_minor: 39_900,
                    currency: 'cny',
                    period_start: 1_700_000_000,
                    period_end: 1_702_419_200,
                    created_at: 1_700_000_000,
                    livemode: false,
                  },
                ],
                billing_debt: 0,
              },
            }

      return {
        data,
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }

    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <SubscriptionPlansCard topupInfo={null} />
        </I18nextProvider>
      )
    })
    await act(async () => {
      await requestsComplete
      await Promise.resolve()
    })

    const text = container.textContent || ''
    assert.match(text, /My Subscriptions/)
    assert.match(text, /Stripe billing/)
    assert.match(text, /Billing history/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('keeps plans visible but disables plan purchases while a subscription is active', async () => {
    let completedRequests = 0
    let resolveRequests: (() => void) | undefined
    const requestsComplete = new Promise<void>((resolve) => {
      resolveRequests = resolve
    })

    api.defaults.adapter = async (config) => {
      completedRequests += 1
      if (completedRequests === 2) resolveRequests?.()

      const data =
        config.url === '/api/subscription/plans'
          ? {
              success: true,
              data: [
                availablePlan,
                {
                  plan: {
                    ...availablePlan.plan,
                    id: 2,
                    title: 'Professional',
                    price_amount: 1_799,
                  },
                },
              ],
            }
          : {
              success: true,
              data: {
                billing_preference: 'subscription_first',
                subscriptions: [activeSubscription],
                all_subscriptions: [activeSubscription],
                stripe_subscriptions: [],
                stripe_invoices: [],
                billing_debt: 0,
              },
            }

      return {
        data,
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }

    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <SubscriptionPlansCard topupInfo={null} />
        </I18nextProvider>
      )
    })
    await act(async () => {
      await requestsComplete
      await Promise.resolve()
    })

    const buttons = [...container.querySelectorAll('button')]
    const currentPlanButton = buttons.find((button) =>
      button.textContent?.includes('Current plan')
    )
    const unavailableButton = buttons.find((button) =>
      button.textContent?.includes('Plan change unavailable')
    )
    assert.ok(currentPlanButton)
    assert.ok(unavailableButton)
    assert.equal(currentPlanButton.disabled, true)
    assert.equal(unavailableButton.disabled, true)
    assert.match(container.textContent || '', /Standard/)
    assert.match(container.textContent || '', /Professional/)
    assert.match(
      container.textContent || '',
      /Plan changes are not supported while you have an active subscription\./
    )

    await act(async () => root.unmount())
    container.remove()
  })

  test('does not offer wallet or subscription charge-order controls', async () => {
    let completedRequests = 0
    let resolveRequests: (() => void) | undefined
    const requestsComplete = new Promise<void>((resolve) => {
      resolveRequests = resolve
    })

    api.defaults.adapter = async (config) => {
      completedRequests += 1
      if (completedRequests === 2) resolveRequests?.()

      const data =
        config.url === '/api/subscription/plans'
          ? { success: true, data: [availablePlan] }
          : {
              success: true,
              data: {
                billing_preference: 'subscription_first',
                subscriptions: [activeSubscription],
                all_subscriptions: [activeSubscription],
                stripe_subscriptions: [],
                stripe_invoices: [],
                billing_debt: 0,
              },
            }

      return {
        data,
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }

    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <SubscriptionPlansCard topupInfo={null} />
        </I18nextProvider>
      )
    })
    await act(async () => {
      await requestsComplete
      await Promise.resolve()
    })

    const text = container.textContent || ''
    assert.doesNotMatch(text, /Wallet First/)
    assert.doesNotMatch(text, /Wallet Only/)
    assert.doesNotMatch(text, /Subscription First/)
    assert.doesNotMatch(text, /Subscription Only/)

    await act(async () => root.unmount())
    container.remove()
  })
})
