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
  resources: {
    en: {
      translation: {
        '{{discount}}/10 price': '{{discount}}/10 price',
        Quota: 'Quota',
        'Total Quota': 'Total Quota',
        'Weekly Quota': 'Weekly Quota',
        'Weekly quota {{quota}}': 'Weekly quota {{quota}}',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

afterEach(() => {
  api.defaults.adapter = originalAdapter
  document.body.replaceChildren()
})

after(() => {
  domWindow.close()
})

describe('subscription plans quota copy', () => {
  test('translates configured plan names everywhere with the active locale', async () => {
    const localizedI18n = createInstance()
    await localizedI18n.use(initReactI18next).init({
      lng: 'zh',
      resources: {
        zh: {
          translation: {
            Standard: '标准',
            'For focused individual development': '适合专注开发的个人',
          },
        },
      },
    })

    let completedRequests = 0
    let resolveRequests: (() => void) | undefined
    const requestsComplete = new Promise<void>((resolve) => {
      resolveRequests = resolve
    })

    api.defaults.adapter = async (config) => {
      completedRequests += 1
      if (completedRequests === 2) resolveRequests?.()

      const subscription = {
        id: 1,
        user_id: 1,
        plan_id: 1,
        status: 'active',
        start_time: 1_700_000_000,
        end_time: 1_900_000_000,
        amount_total: 55_000_000,
        amount_used: 0,
      }
      const data =
        config.url === '/api/subscription/plans'
          ? {
              success: true,
              data: [
                {
                  plan: {
                    id: 1,
                    title: 'Standard',
                    subtitle: 'For focused individual development',
                    price_amount: 399,
                    currency: 'CNY',
                    duration_unit: 'day',
                    duration_value: 28,
                    quota_reset_period: 'custom',
                    quota_reset_custom_seconds: 604_800,
                    max_purchase_per_user: 0,
                    total_amount: 55_000_000,
                  },
                },
              ],
            }
          : {
              success: true,
              data: {
                billing_preference: 'subscription_first',
                subscriptions: [{ subscription }],
                all_subscriptions: [{ subscription }],
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
        <I18nextProvider i18n={localizedI18n}>
          <SubscriptionPlansCard topupInfo={null} />
        </I18nextProvider>
      )
    })
    await act(async () => {
      await requestsComplete
      await Promise.resolve()
    })

    const text = container.textContent || ''
    assert.match(text, /标准/)
    assert.match(text, /适合专注开发的个人/)
    assert.doesNotMatch(text, /Standard/)
    assert.doesNotMatch(text, /For focused individual development/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('describes resettable subscription allowance as weekly quota', async () => {
    let completedRequests = 0
    let resolveRequests: (() => void) | undefined
    const requestsComplete = new Promise<void>((resolve) => {
      resolveRequests = resolve
    })

    api.defaults.adapter = async (config) => {
      completedRequests += 1
      if (completedRequests === 2) resolveRequests?.()

      const subscription = {
        id: 1,
        user_id: 1,
        plan_id: 1,
        status: 'active',
        start_time: 1_700_000_000,
        end_time: 1_900_000_000,
        amount_total: 55_000_000,
        amount_used: 5_000_000,
      }
      const data =
        config.url === '/api/subscription/plans'
          ? {
              success: true,
              data: [
                {
                  plan: {
                    id: 1,
                    title: 'Standard',
                    price_amount: 399,
                    currency: 'CNY',
                    duration_unit: 'day',
                    duration_value: 28,
                    quota_reset_period: 'custom',
                    quota_reset_custom_seconds: 604_800,
                    max_purchase_per_user: 0,
                    total_amount: 55_000_000,
                  },
                },
                {
                  plan: {
                    id: 2,
                    title: 'Monthly allowance',
                    price_amount: 199,
                    currency: 'CNY',
                    duration_unit: 'month',
                    duration_value: 1,
                    quota_reset_period: 'monthly',
                    quota_reset_custom_seconds: 0,
                    max_purchase_per_user: 0,
                    total_amount: 20_000_000,
                  },
                },
              ],
            }
          : {
              success: true,
              data: {
                billing_preference: 'subscription_first',
                subscriptions: [{ subscription }],
                all_subscriptions: [{ subscription }],
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
    assert.match(text, /Weekly Quota:/)
    assert.doesNotMatch(text, /Total Quota/)

    const cardTextByTitle = new Map<string, string>()
    for (const heading of container.querySelectorAll('h4')) {
      const card = heading.closest('[data-slot="card"]')
      cardTextByTitle.set(heading.textContent || '', card?.textContent || '')
    }
    assert.match(cardTextByTitle.get('Standard') || '', /Weekly quota \$110/)
    assert.match(cardTextByTitle.get('Monthly allowance') || '', /Quota: \$40/)
    assert.doesNotMatch(
      cardTextByTitle.get('Monthly allowance') || '',
      /Weekly quota/
    )

    await act(async () => root.unmount())
    container.remove()
  })

  test('shows one-decimal discounts rounded down from each price versus four weekly quotas', async () => {
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
                {
                  plan: {
                    id: 1,
                    title: 'Standard',
                    price_amount: 399,
                    currency: 'CNY',
                    duration_unit: 'day',
                    duration_value: 28,
                    quota_reset_period: 'custom',
                    quota_reset_custom_seconds: 604_800,
                    max_purchase_per_user: 0,
                    total_amount: 55_000_000,
                  },
                },
                {
                  plan: {
                    id: 2,
                    title: 'Premium',
                    price_amount: 899,
                    currency: 'CNY',
                    duration_unit: 'day',
                    duration_value: 28,
                    quota_reset_period: 'custom',
                    quota_reset_custom_seconds: 604_800,
                    max_purchase_per_user: 0,
                    total_amount: 130_000_000,
                  },
                },
                {
                  plan: {
                    id: 3,
                    title: 'Professional',
                    price_amount: 1_799,
                    currency: 'CNY',
                    duration_unit: 'day',
                    duration_value: 28,
                    quota_reset_period: 'custom',
                    quota_reset_custom_seconds: 604_800,
                    max_purchase_per_user: 0,
                    total_amount: 265_000_000,
                  },
                },
                {
                  plan: {
                    id: 4,
                    title: 'Thirty day plan',
                    price_amount: 100,
                    currency: 'CNY',
                    duration_unit: 'day',
                    duration_value: 30,
                    quota_reset_period: 'custom',
                    quota_reset_custom_seconds: 604_800,
                    max_purchase_per_user: 0,
                    total_amount: 55_000_000,
                  },
                },
              ],
            }
          : {
              success: true,
              data: {
                billing_preference: 'subscription_first',
                subscriptions: [],
                all_subscriptions: [],
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

    const cardTextByTitle = new Map<string, string>()
    for (const heading of container.querySelectorAll('h4')) {
      const card = heading.closest('[data-slot="card"]')
      cardTextByTitle.set(heading.textContent || '', card?.textContent || '')
    }

    assert.match(cardTextByTitle.get('Standard') || '', /9\/10 price/)
    assert.match(cardTextByTitle.get('Premium') || '', /8\.6\/10 price/)
    assert.match(cardTextByTitle.get('Professional') || '', /8\.4\/10 price/)
    assert.doesNotMatch(
      cardTextByTitle.get('Thirty day plan') || '',
      /\/10 price/
    )
    assert.doesNotMatch(container.textContent || '', /You save|%/)

    await act(async () => root.unmount())
    container.remove()
  })
})
