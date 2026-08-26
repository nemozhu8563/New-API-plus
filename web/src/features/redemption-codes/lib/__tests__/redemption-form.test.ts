import assert from 'node:assert/strict'

import type { TFunction } from 'i18next'
import { describe, test } from 'vitest'

import {
  getRedemptionFormSchema,
  truncateRedemptionName,
  transformFormDataToPayload,
  transformRedemptionToFormDefaults,
} from '../redemption-form'

const t = ((key: string) => key) as TFunction

describe('redemption form benefit types', () => {
  test('quota codes require a positive quota and preserve the legacy payload', () => {
    const schema = getRedemptionFormSchema(t)
    const invalid = schema.safeParse({
      name: 'quota code',
      benefit_type: 'quota',
      quota_dollars: 0,
      subscription_plan_id: '',
      count: 1,
    })
    assert.equal(invalid.success, false)

    const payload = transformFormDataToPayload({
      name: 'quota code',
      benefit_type: 'quota',
      quota_dollars: 10,
      subscription_plan_id: '',
      count: 2,
    })
    assert.equal(payload.benefit_type, 'quota')
    assert.equal(payload.quota > 0, true)
    assert.equal(payload.subscription_plan_id, undefined)
    assert.equal(payload.count, 2)
  })

  test('subscription codes require a plan and never send wallet quota', () => {
    const schema = getRedemptionFormSchema(t)
    const invalid = schema.safeParse({
      name: 'subscription code',
      benefit_type: 'subscription',
      quota_dollars: 0,
      subscription_plan_id: '',
      count: 1,
    })
    assert.equal(invalid.success, false)

    const payload = transformFormDataToPayload({
      name: 'subscription code',
      benefit_type: 'subscription',
      quota_dollars: 0,
      subscription_plan_id: '42',
      count: 1,
    })
    assert.deepEqual(payload, {
      name: 'subscription code',
      benefit_type: 'subscription',
      quota: 0,
      subscription_plan_id: 42,
      expired_time: 0,
      count: 1,
    })
  })

  test('subscription plan ids must be positive safe integers', () => {
    const schema = getRedemptionFormSchema(t)
    for (const subscriptionPlanId of ['1.5', '9007199254740992', '-1']) {
      const result = schema.safeParse({
        name: 'subscription code',
        benefit_type: 'subscription',
        quota_dollars: 0,
        subscription_plan_id: subscriptionPlanId,
        count: 1,
      })
      assert.equal(result.success, false)
    }
  })

  test('generated subscription names fit the redemption name limit', () => {
    const name = truncateRedemptionName('  Professional subscription plan  ')
    assert.equal(name, 'Professional subscri')
    assert.equal(name.length, 20)

    const emojiName = truncateRedemptionName('1234567890123456789🚀extra')
    assert.equal(emojiName, '1234567890123456789')
    assert.equal(emojiName.length, 19)
  })

  test('legacy redemption records default to quota form values', () => {
    const values = transformRedemptionToFormDefaults({
      id: 1,
      user_id: 1,
      name: 'legacy',
      key: 'legacy-key',
      status: 1,
      quota: 500,
      benefit_type: 'quota',
      subscription_plan_id: 0,
      subscription_plan_title: '',
      used_subscription_id: 0,
      created_time: 1,
      redeemed_time: 0,
      expired_time: 0,
      used_user_id: 0,
    })
    assert.equal(values.benefit_type, 'quota')
    assert.equal(values.subscription_plan_id, '')
  })
})
