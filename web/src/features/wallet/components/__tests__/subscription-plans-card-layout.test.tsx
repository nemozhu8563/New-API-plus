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

afterEach(() => {
  api.defaults.adapter = originalAdapter
  document.body.replaceChildren()
})

after(() => {
  domWindow.close()
})

describe('subscription plans layout', () => {
  test('shows three prominent plan cards in one row from medium screens upward', async () => {
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
                    title: 'Starter',
                    price_amount: 99,
                    currency: 'CNY',
                    duration_unit: 'day',
                    duration_value: 28,
                    quota_reset_period: 'weekly',
                    max_purchase_per_user: 0,
                    total_amount: 100,
                  },
                },
                {
                  plan: {
                    id: 2,
                    title: 'Standard',
                    price_amount: 399,
                    currency: 'CNY',
                    duration_unit: 'day',
                    duration_value: 28,
                    quota_reset_period: 'weekly',
                    max_purchase_per_user: 0,
                    total_amount: 110,
                  },
                },
                {
                  plan: {
                    id: 3,
                    title: 'Premium',
                    price_amount: 899,
                    currency: 'CNY',
                    duration_unit: 'day',
                    duration_value: 28,
                    quota_reset_period: 'weekly',
                    max_purchase_per_user: 0,
                    total_amount: 260,
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

    const planGrid = container.querySelector(
      '[data-slot="subscription-plans-grid"]'
    )
    assert.ok(planGrid)
    assert.ok(planGrid.classList.contains('grid-cols-1'))
    assert.ok(planGrid.classList.contains('sm:grid-cols-2'))
    assert.ok(planGrid.classList.contains('md:grid-cols-3'))

    const planCards = [...planGrid.children]
    assert.equal(planCards.length, 3)
    for (const planCard of planCards) {
      assert.ok(planCard.classList.contains('min-h-[340px]'))

      const content = planCard.querySelector('[data-slot="card-content"]')
      assert.ok(content)
      assert.ok(content.classList.contains('p-5'))
      assert.ok(content.classList.contains('sm:p-6'))

      const title = planCard.querySelector('h4')
      assert.ok(title)
      assert.ok(title.classList.contains('text-lg'))
      assert.ok(planCard.querySelector('.text-3xl'))

      const subscribeButton = planCard.querySelector('[data-slot="button"]')
      assert.ok(subscribeButton)
      assert.ok(subscribeButton.classList.contains('h-9'))
    }

    await act(async () => root.unmount())
    container.remove()
  })
})
