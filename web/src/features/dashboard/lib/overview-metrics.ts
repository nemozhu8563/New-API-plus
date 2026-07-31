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
import type { PerfModelSummary } from '@/features/performance-metrics/types'

export function getWeightedSuccessRate(
  models: PerfModelSummary[]
): number | null {
  let weightedTotal = 0
  let totalRequests = 0
  const validRates: number[] = []

  for (const model of models) {
    const successRate = Number(model.success_rate)
    if (!Number.isFinite(successRate)) continue

    validRates.push(successRate)
    const requestCount = Number(model.request_count)
    if (Number.isFinite(requestCount) && requestCount > 0) {
      weightedTotal += successRate * requestCount
      totalRequests += requestCount
    }
  }

  if (totalRequests > 0) {
    return weightedTotal / totalRequests
  }
  if (validRates.length === 0) {
    return null
  }
  return validRates.reduce((total, rate) => total + rate, 0) / validRates.length
}

export function getErrorRate(models: PerfModelSummary[]): number | null {
  const successRate = getWeightedSuccessRate(models)
  if (successRate === null) return null
  return Math.min(100, Math.max(0, 100 - successRate))
}
