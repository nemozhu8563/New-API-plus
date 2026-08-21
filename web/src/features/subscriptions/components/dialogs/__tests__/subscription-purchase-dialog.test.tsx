import assert from 'node:assert/strict'
import { after, test } from 'node:test'

import { Window } from 'happy-dom'

import type { PublicPlanRecord } from '@/features/subscriptions/types'

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
const { SubscriptionPurchaseDialog } =
  await import('../subscription-purchase-dialog')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        Pay: 'Pay',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const stripePlan: PublicPlanRecord = {
  plan: {
    id: 1,
    title: 'Test plan',
    price_amount: 20,
    currency: 'USD',
    duration_unit: 'month',
    duration_value: 1,
    quota_reset_period: 'never',
    max_purchase_per_user: 0,
    total_amount: 1_000_000,
    stripe_checkout_available: true,
    creem_checkout_available: false,
    waffo_checkout_available: false,
  },
}

after(() => {
  domWindow.close()
})

test('labels the Stripe Checkout action as Pay', async () => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <SubscriptionPurchaseDialog
          open
          onOpenChange={() => undefined}
          plan={stripePlan}
          enableStripe
        />
      </I18nextProvider>
    )
  })

  const buttonLabels = new Set(
    [...document.querySelectorAll('button')].map((button) => button.textContent)
  )
  assert.ok(buttonLabels.has('Pay'))
  assert.equal(buttonLabels.has('Stripe'), false)

  await act(async () => root.unmount())
  container.remove()
})
