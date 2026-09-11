import type { PerfModelSummary } from '@/features/performance-metrics/types'
import type { UserSubscriptionRecord } from '@/features/subscriptions/types'

export function getRemainingSubscriptionQuota(
  subscriptions: UserSubscriptionRecord[]
): number {
  return subscriptions.reduce((remainingQuota, record) => {
    const total = Number(record.subscription.amount_total)
    if (!Number.isFinite(total) || total <= 0) return remainingQuota

    const used = Number(record.subscription.amount_used)
    const normalizedUsed = Number.isFinite(used) ? Math.max(0, used) : 0
    return remainingQuota + Math.max(0, total - normalizedUsed)
  }, 0)
}

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
