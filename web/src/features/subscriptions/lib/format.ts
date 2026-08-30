import type { TFunction } from 'i18next'

import dayjs from '@/lib/dayjs'

import type { SubscriptionPlan } from '../types'

export function formatSubscriptionPrice(
  amount: number,
  currency = 'CNY'
): string {
  const normalizedAmount = Number.isFinite(amount) ? amount : 0
  return Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: currency.trim().toUpperCase() || 'CNY',
    currencyDisplay: 'narrowSymbol',
    minimumFractionDigits: Number.isInteger(normalizedAmount) ? 0 : 2,
    maximumFractionDigits: 2,
  }).format(normalizedAmount)
}

const planAllowanceSubtitlePattern =
  /^Includes\s+(?:\$\s*)?[\d,.]+(?:\s+(?:USD|Credits?))?\s+per billing cycle$/i

export function formatSubscriptionPlanSubtitle(
  plan: Pick<SubscriptionPlan, 'subtitle' | 'total_amount'>,
  quotaPerUnit: number,
  t: TFunction
): string {
  const subtitle = plan.subtitle?.trim() || ''
  if (!planAllowanceSubtitlePattern.test(subtitle)) return subtitle

  const totalAmount = Number(plan.total_amount || 0)
  const monthlyAllowance = totalAmount / quotaPerUnit
  if (!Number.isFinite(monthlyAllowance) || monthlyAllowance <= 0) {
    return subtitle
  }

  return t('Includes {{amount}} per billing cycle', {
    amount: formatSubscriptionPrice(monthlyAllowance, 'USD'),
  })
}

export function formatDuration(
  plan: Partial<SubscriptionPlan>,
  t: TFunction
): string {
  const unit = plan?.duration_unit || 'month'
  const value = plan?.duration_value || 1
  const unitLabels: Record<string, string> = {
    year: t('years'),
    month: t('months'),
    day: t('days'),
    hour: t('hours'),
    custom: t('Custom (seconds)'),
  }
  if (unit === 'custom') {
    const seconds = plan?.custom_seconds || 0
    if (seconds >= 86400) return `${Math.floor(seconds / 86400)} ${t('days')}`
    if (seconds >= 3600) return `${Math.floor(seconds / 3600)} ${t('hours')}`
    return `${seconds} ${t('seconds')}`
  }
  return `${value} ${unitLabels[unit] || unit}`
}

export function formatResetPeriod(
  plan: Partial<SubscriptionPlan>,
  t: TFunction
): string {
  const period = plan?.quota_reset_period || 'never'
  if (period === 'daily') return t('Daily')
  if (period === 'weekly') return t('Weekly')
  if (period === 'monthly') return t('Monthly')
  if (period === 'billing_cycle') return t('Each billing cycle')
  if (period === 'custom') {
    const seconds = Number(plan?.quota_reset_custom_seconds || 0)
    if (seconds >= 86400) return `${Math.floor(seconds / 86400)} ${t('days')}`
    if (seconds >= 3600) return `${Math.floor(seconds / 3600)} ${t('hours')}`
    if (seconds >= 60) return `${Math.floor(seconds / 60)} ${t('minutes')}`
    return `${seconds} ${t('seconds')}`
  }
  return t('No Reset')
}

export function formatTimestamp(ts: number): string {
  if (!ts) return '-'
  return dayjs(ts * 1000).format('YYYY-MM-DD HH:mm:ss')
}
