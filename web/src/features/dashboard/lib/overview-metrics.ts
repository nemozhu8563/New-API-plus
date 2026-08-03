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
