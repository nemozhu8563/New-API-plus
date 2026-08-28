import assert from 'node:assert/strict'

import { beforeEach, describe, test } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import type { SubscriptionPlan } from '../../types'
import {
  formValuesToPlanPayload,
  PLAN_FORM_DEFAULTS,
  planToFormValues,
} from '../plan-form'

beforeEach(() => {
  useSystemConfigStore.getState().setConfig({
    currency: { ...DEFAULT_CURRENCY_CONFIG },
  })
})

describe('subscription plan form', () => {
  test('round-trips public visibility, recommendation, and monthly quota', () => {
    const plan: SubscriptionPlan = {
      id: 7,
      title: 'Professional',
      subtitle: 'For production workloads',
      price_amount: 699,
      currency: 'CNY',
      duration_unit: 'month',
      duration_value: 1,
      custom_seconds: 0,
      quota_reset_period: 'billing_cycle',
      quota_reset_custom_seconds: 0,
      enabled: true,
      public_visible: false,
      recommended: true,
      sort_order: 30,
      allow_wallet_overflow: false,
      max_purchase_per_user: 2,
      total_amount: 55_000_000,
      upgrade_group: 'professional',
      downgrade_group: 'default',
      stripe_price_id: 'price_professional',
      creem_product_id: '',
      waffo_pancake_product_id: '',
    }

    const values = planToFormValues(plan)
    assert.equal(values.public_visible, false)
    assert.equal(values.recommended, true)
    assert.equal(values.total_amount, 110)

    const payload = formValuesToPlanPayload(values).plan
    assert.equal(payload.public_visible, false)
    assert.equal(payload.recommended, true)
    assert.equal(payload.total_amount, 55_000_000)
  })

  test('always emits the supported monthly billing contract', () => {
    const payload = formValuesToPlanPayload({
      ...PLAN_FORM_DEFAULTS,
      title: 'Starter',
      total_amount: 80,
    }).plan

    assert.equal(payload.currency, 'CNY')
    assert.equal(payload.duration_unit, 'month')
    assert.equal(payload.duration_value, 1)
    assert.equal(payload.custom_seconds, 0)
    assert.equal(payload.quota_reset_period, 'billing_cycle')
    assert.equal(payload.quota_reset_custom_seconds, 0)
    assert.equal(payload.total_amount, 40_000_000)
  })
})
