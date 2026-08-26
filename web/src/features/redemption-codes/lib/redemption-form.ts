import type { TFunction } from 'i18next'
import { z } from 'zod'

import { parseQuotaFromDollars, quotaUnitsToEditableAmount } from '@/lib/format'

import {
  REDEMPTION_VALIDATION,
  getRedemptionFormErrorMessages,
} from '../constants'
import type {
  Redemption,
  RedemptionBenefitType,
  RedemptionFormData,
} from '../types'

// ============================================================================
// Form Schema (use getRedemptionFormSchema(t) in components for i18n messages)
// ============================================================================

export function getRedemptionFormSchema(t: TFunction) {
  const msg = getRedemptionFormErrorMessages(t)
  return z
    .object({
      name: z
        .string()
        .min(REDEMPTION_VALIDATION.NAME_MIN_LENGTH, msg.NAME_LENGTH_INVALID)
        .max(REDEMPTION_VALIDATION.NAME_MAX_LENGTH, msg.NAME_LENGTH_INVALID),
      benefit_type: z.enum(['quota', 'subscription']),
      quota_dollars: z.number().min(0),
      subscription_plan_id: z.string(),
      expired_time: z.date().optional(),
      count: z
        .number()
        .min(REDEMPTION_VALIDATION.COUNT_MIN, msg.COUNT_INVALID)
        .max(REDEMPTION_VALIDATION.COUNT_MAX, msg.COUNT_INVALID)
        .optional(),
    })
    .superRefine((data, context) => {
      if (data.benefit_type === 'quota' && data.quota_dollars <= 0) {
        context.addIssue({
          code: 'custom',
          path: ['quota_dollars'],
          message: t('Quota must be a positive number'),
        })
      }
      if (
        data.benefit_type === 'subscription' &&
        (!data.subscription_plan_id ||
          !Number.isSafeInteger(Number(data.subscription_plan_id)) ||
          Number(data.subscription_plan_id) <= 0)
      ) {
        context.addIssue({
          code: 'custom',
          path: ['subscription_plan_id'],
          message: t('Please select a subscription plan'),
        })
      }
    })
}

export function truncateRedemptionName(name: string): string {
  let truncated = ''
  for (const character of name.trim()) {
    if (
      (truncated + character).length > REDEMPTION_VALIDATION.NAME_MAX_LENGTH
    ) {
      break
    }
    truncated += character
  }
  return truncated
}

export type RedemptionFormValues = {
  name: string
  benefit_type: RedemptionBenefitType
  quota_dollars: number
  subscription_plan_id: string
  expired_time?: Date
  count?: number
}

// ============================================================================
// Form Defaults
// ============================================================================

export const REDEMPTION_FORM_DEFAULT_VALUES: RedemptionFormValues = {
  name: '',
  benefit_type: 'quota',
  quota_dollars: 10,
  subscription_plan_id: '',
  expired_time: undefined,
  count: 1,
}

// ============================================================================
// Form Data Transformation
// ============================================================================

/**
 * Transform form data to API payload
 */
export function transformFormDataToPayload(
  data: RedemptionFormValues
): RedemptionFormData {
  return {
    name: data.name,
    benefit_type: data.benefit_type,
    quota:
      data.benefit_type === 'quota'
        ? parseQuotaFromDollars(data.quota_dollars)
        : 0,
    subscription_plan_id:
      data.benefit_type === 'subscription'
        ? Number(data.subscription_plan_id)
        : undefined,
    expired_time: data.expired_time
      ? Math.floor(data.expired_time.getTime() / 1000)
      : 0,
    count: data.count || 1,
  }
}

/**
 * Transform redemption data to form defaults
 */
export function transformRedemptionToFormDefaults(
  redemption: Redemption
): RedemptionFormValues {
  return {
    name: redemption.name,
    benefit_type: redemption.benefit_type || 'quota',
    quota_dollars: quotaUnitsToEditableAmount(redemption.quota),
    subscription_plan_id:
      redemption.subscription_plan_id > 0
        ? String(redemption.subscription_plan_id)
        : '',
    expired_time:
      redemption.expired_time > 0
        ? new Date(redemption.expired_time * 1000)
        : undefined,
    count: 1,
  }
}
