import assert from 'node:assert/strict'
import { after, afterEach, describe, test } from 'node:test'

import { Window } from 'happy-dom'

import type {
  StripeInvoiceSummary,
  StripeSubscriptionSummary,
} from '@/features/subscriptions/types'

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
const { StripeSubscriptionBilling } =
  await import('../stripe-subscription-billing')
const { formatStripeInvoiceAmount } = await import('../stripe-invoice-amount')

const originalAdapter = api.defaults.adapter
const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        active: 'active',
        'Access ends on {{date}}': 'Access ends on {{date}}',
        'Automatic renewal will stop. Your paid subscription and its quota remain available until {{date}}.':
          'Automatic renewal will stop. Your paid subscription and its quota remain available until {{date}}.',
        'Billing history': 'Billing history',
        Cancel: 'Cancel',
        'Cancel Stripe subscription?': 'Cancel Stripe subscription?',
        'Cancel subscription': 'Cancel subscription',
        'Cancellation scheduled': 'Cancellation scheduled',
        'Cancelling...': 'Cancelling...',
        'Confirm cancellation': 'Confirm cancellation',
        'Next billing date: {{date}}': 'Next billing date: {{date}}',
        'No Stripe invoices yet': 'No Stripe invoices yet',
        'Opening billing portal...': 'Opening billing portal...',
        'Payment methods & invoices': 'Payment methods & invoices',
        'Review renewal and cancellation status here. Use Stripe only to update payment methods or open complete invoices.':
          'Review renewal and cancellation status here. Use Stripe only to update payment methods or open complete invoices.',
        'Showing the 8 most recent invoices. Open Stripe for complete history.':
          'Showing the 8 most recent invoices. Open Stripe for complete history.',
        'Stripe billing': 'Stripe billing',
        'Subscription will end after the current billing period':
          'Subscription will end after the current billing period',
        'Unable to cancel subscription': 'Unable to cancel subscription',
        'Unable to open billing portal': 'Unable to open billing portal',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const activeSubscription: StripeSubscriptionSummary = {
  subscription_id: 'sub_test_123',
  customer_id: 'cus_test_123',
  plan_id: 1,
  plan_title: 'Builder',
  status: 'active',
  cancel_at_period_end: false,
  cancel_at: 0,
  current_period_end: 1_800_000_000,
  livemode: false,
}

async function renderBilling(
  subscription: StripeSubscriptionSummary,
  onRefresh: () => Promise<void>,
  invoices: StripeInvoiceSummary[] = []
) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <StripeSubscriptionBilling
          subscriptions={[subscription]}
          invoices={invoices}
          billingDebt={0}
          onRefresh={onRefresh}
        />
      </I18nextProvider>
    )
  })
  return { container, root }
}

function findButton(label: string): HTMLButtonElement {
  const button = [...document.body.querySelectorAll('button')].find(
    (candidate) => candidate.textContent?.includes(label)
  )
  assert.ok(button, `Expected button containing ${label}`)
  return button
}

afterEach(() => {
  api.defaults.adapter = originalAdapter
  document.body.replaceChildren()
})

after(() => {
  domWindow.close()
})

describe('Stripe subscription billing', () => {
  test('confirms period-end cancellation and refreshes billing state', async () => {
    let requestUrl = ''
    let requestMethod = ''
    let requestBody: unknown
    let refreshCount = 0
    api.defaults.adapter = async (config) => {
      requestUrl = config.url || ''
      requestMethod = config.method || ''
      requestBody = JSON.parse(String(config.data))
      return {
        data: { success: true, data: { ...activeSubscription } },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }

    const { container, root } = await renderBilling(
      activeSubscription,
      async () => {
        refreshCount += 1
      }
    )

    await act(async () => findButton('Cancel subscription').click())
    await act(async () => {
      findButton('Confirm cancellation').click()
      await Promise.resolve()
      await Promise.resolve()
    })

    assert.equal(requestUrl, '/api/subscription/stripe/cancel')
    assert.equal(requestMethod, 'post')
    assert.deepEqual(requestBody, { subscription_id: 'sub_test_123' })
    assert.equal(refreshCount, 1)

    await act(async () => root.unmount())
    container.remove()
  })

  test('shows period-end access and hides cancellation after it is scheduled', async () => {
    const scheduled = {
      ...activeSubscription,
      cancel_at_period_end: true,
    }
    const { container, root } = await renderBilling(scheduled, async () => {})

    assert.equal(
      container.textContent?.includes('Cancellation scheduled'),
      true
    )
    assert.equal(container.textContent?.includes('Access ends on'), true)
    assert.equal(
      [...container.querySelectorAll('button')].some((button) =>
        button.textContent?.includes('Cancel subscription')
      ),
      false
    )

    await act(async () => root.unmount())
    container.remove()
  })

  test('formats Stripe minor-unit invoice amounts using the invoice currency', () => {
    assert.equal(formatStripeInvoiceAmount(2_000, 'cny', 'en-US'), '¥20.00')
    assert.equal(formatStripeInvoiceAmount(2_000, 'jpy', 'ja-JP'), '￥2,000')
  })

  test('labels billing history as the eight most recent invoices', async () => {
    const invoices = Array.from({ length: 10 }, (_, index) => ({
      invoice_id: `in_test_${index}`,
      subscription_id: activeSubscription.subscription_id,
      plan_title: `Invoice ${index}`,
      amount_paid_minor: 2_000,
      currency: 'cny',
      period_start: 1_700_000_000 + index,
      period_end: 1_700_086_400 + index,
      created_at: 1_700_086_400 + index,
      livemode: false,
    }))
    const { container, root } = await renderBilling(
      activeSubscription,
      async () => {},
      invoices
    )

    assert.equal(
      container.textContent?.includes(
        'Showing the 8 most recent invoices. Open Stripe for complete history.'
      ),
      true
    )
    assert.equal(container.querySelectorAll('[role="listitem"]').length, 8)

    await act(async () => root.unmount())
    container.remove()
  })
})
