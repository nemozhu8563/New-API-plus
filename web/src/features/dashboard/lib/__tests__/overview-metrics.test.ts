import assert from 'node:assert/strict'

import { describe, test } from 'vitest'

import type { PerfModelSummary } from '@/features/performance-metrics/types'
import type { UserSubscriptionRecord } from '@/features/subscriptions/types'

import {
  getErrorRate,
  getRemainingSubscriptionQuota,
  getWeightedSuccessRate,
} from '../overview-metrics'

function model(successRate: number, requestCount?: number): PerfModelSummary {
  return {
    model_name: `model-${successRate}-${requestCount ?? 'none'}`,
    avg_latency_ms: 100,
    avg_tps: 10,
    success_rate: successRate,
    request_count: requestCount,
  }
}

function subscription(
  amountTotal: number,
  amountUsed: number
): UserSubscriptionRecord {
  return {
    subscription: {
      id: amountTotal + amountUsed,
      user_id: 1,
      plan_id: 1,
      status: 'active',
      start_time: 1,
      end_time: 2,
      amount_total: amountTotal,
      amount_used: amountUsed,
    },
  }
}

describe('overview subscription quota', () => {
  test('adds the remaining quota from every active subscription', () => {
    const subscriptions = [
      subscription(1_000_000, 200_000),
      subscription(500_000, 300_000),
    ]

    assert.equal(getRemainingSubscriptionQuota(subscriptions), 1_000_000)
  })

  test('does not let an overused or malformed subscription reduce the total', () => {
    const subscriptions = [
      subscription(500_000, 700_000),
      subscription(300_000, -100_000),
      subscription(Number.NaN, 0),
    ]

    assert.equal(getRemainingSubscriptionQuota(subscriptions), 300_000)
    assert.equal(getRemainingSubscriptionQuota([]), 0)
  })
})

describe('overview performance metrics', () => {
  test('weights success rate by request volume', () => {
    const models = [model(100, 90), model(50, 10)]

    assert.equal(getWeightedSuccessRate(models), 95)
    assert.equal(getErrorRate(models), 5)
  })

  test('falls back to a simple average when request counts are unavailable', () => {
    assert.equal(getWeightedSuccessRate([model(98), model(96)]), 97)
    assert.equal(getErrorRate([model(98), model(96)]), 3)
  })

  test('returns no metric without valid model data and clamps invalid bounds', () => {
    assert.equal(getErrorRate([]), null)
    assert.equal(getErrorRate([model(120, 1)]), 0)
    assert.equal(getErrorRate([model(-20, 1)]), 100)
  })
})
