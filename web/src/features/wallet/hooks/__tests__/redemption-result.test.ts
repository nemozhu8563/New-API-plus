import assert from 'node:assert/strict'

import i18next from 'i18next'
import { beforeAll as before, describe, test } from 'vitest'

import { formatQuota } from '@/lib/format'

import { getRedemptionSuccessMessage } from '../use-redemption'

before(async () => {
  await i18next.init({
    lng: 'en',
    resources: {
      en: {
        translation: {
          'Redemption successful! Added: {{quota}}':
            'Redemption successful! Added: {{quota}}',
          'Subscription activated: {{plan}}':
            'Subscription activated: {{plan}}',
        },
      },
    },
  })
})

describe('redemption result messages', () => {
  test('quota result reports the credited wallet amount', () => {
    assert.equal(
      getRedemptionSuccessMessage({ type: 'quota', quota: 500 }),
      `Redemption successful! Added: ${formatQuota(500)}`
    )
  })

  test('subscription result reports the activated frozen plan title', () => {
    assert.equal(
      getRedemptionSuccessMessage({
        type: 'subscription',
        subscription_id: 9,
        plan_id: 3,
        plan_title: 'Pro Monthly',
        start_time: 100,
        end_time: 200,
      }),
      'Subscription activated: Pro Monthly'
    )
  })
})
