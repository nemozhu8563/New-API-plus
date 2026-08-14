import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { Window } from 'happy-dom'

import { PAYMENT_TYPES } from '../../constants'
import {
  dispatchSelectedPayment,
  isStripeOnlyTopUp,
  isStripeSubscriptionEnabled,
  isStripePayment,
  isWaffoPayment,
  isWaffoPancakePayment,
  redirectToHostedCheckout,
} from '../payment'

describe('payment type classification', () => {
  test('keeps Waffo and Waffo Pancake on their dedicated flows', () => {
    assert.equal(isWaffoPayment(PAYMENT_TYPES.WAFFO), true)
    assert.equal(isWaffoPayment(PAYMENT_TYPES.WAFFO_PANCAKE), false)
    assert.equal(isWaffoPancakePayment(PAYMENT_TYPES.WAFFO_PANCAKE), true)
    assert.equal(isWaffoPancakePayment(PAYMENT_TYPES.WAFFO), false)
    assert.equal(isStripePayment(PAYMENT_TYPES.STRIPE), true)
  })
})

describe('Stripe subscription availability', () => {
  test('allows subscriptions when Stripe topups are disabled', () => {
    assert.equal(
      isStripeSubscriptionEnabled({
        enable_online_topup: false,
        enable_stripe_topup: false,
        enable_stripe_subscription: true,
        pay_methods: [],
        min_topup: 1,
        stripe_min_topup: 1,
        amount_options: [],
        discount: {},
      }),
      true
    )
  })
})

describe('Stripe-only topup availability', () => {
  test('ignores stale configured methods when Stripe is the only enabled provider', () => {
    assert.equal(
      isStripeOnlyTopUp({
        enable_online_topup: false,
        enable_stripe_topup: true,
        pay_methods: [{ name: 'Stale card', type: 'card' }],
        min_topup: 1,
        stripe_min_topup: 20,
        amount_options: [],
        discount: {},
      }),
      true
    )
  })

  test('keeps the multi-provider flow when another provider is enabled', () => {
    assert.equal(
      isStripeOnlyTopUp({
        enable_online_topup: true,
        enable_stripe_topup: true,
        pay_methods: [{ name: 'Stripe', type: 'stripe' }],
        min_topup: 1,
        stripe_min_topup: 20,
        amount_options: [],
        discount: {},
      }),
      false
    )
  })
})

describe('hosted checkout navigation', () => {
  test('redirects Stripe checkout in the current tab', () => {
    const domWindow = new Window({ url: 'https://app.example.test/wallet' })
    const originalWindow = globalThis.window
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: domWindow,
    })

    try {
      redirectToHostedCheckout('https://checkout.stripe.example/session')
      assert.equal(
        domWindow.location.href,
        'https://checkout.stripe.example/session'
      )
    } finally {
      Object.defineProperty(globalThis, 'window', {
        configurable: true,
        value: originalWindow,
      })
      domWindow.close()
    }
  })
})

describe('payment dispatch', () => {
  test('keeps the selected Waffo method index through confirmation', async () => {
    const calls: string[] = []
    const success = await dispatchSelectedPayment(
      { name: 'Waffo Card', type: PAYMENT_TYPES.WAFFO },
      120,
      3,
      {
        regular: async () => {
          calls.push('regular')
          return false
        },
        waffo: async (amount, index) => {
          calls.push(`waffo:${amount}:${index}`)
          return true
        },
        waffoPancake: async () => {
          calls.push('pancake')
          return false
        },
      }
    )

    assert.equal(success, true)
    assert.deepEqual(calls, ['waffo:120:3'])
  })

  test('does not create a Waffo order without a selected method index', async () => {
    let called = false
    const success = await dispatchSelectedPayment(
      { name: 'Waffo Card', type: PAYMENT_TYPES.WAFFO },
      120,
      null,
      {
        regular: async () => false,
        waffo: async () => {
          called = true
          return true
        },
        waffoPancake: async () => false,
      }
    )

    assert.equal(success, false)
    assert.equal(called, false)
  })
})
