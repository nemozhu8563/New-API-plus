/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { PerfModelSummary } from '@/features/performance-metrics/types'

import { getErrorRate, getWeightedSuccessRate } from '../overview-metrics'

function model(successRate: number, requestCount?: number): PerfModelSummary {
  return {
    model_name: `model-${successRate}-${requestCount ?? 'none'}`,
    avg_latency_ms: 100,
    avg_tps: 10,
    success_rate: successRate,
    request_count: requestCount,
  }
}

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
