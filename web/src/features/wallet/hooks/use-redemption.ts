import i18next from 'i18next'
import { useState, useCallback } from 'react'
import { toast } from 'sonner'

import { getSelf } from '@/lib/api'
import { formatQuota } from '@/lib/format'

import { redeemCode as redeemTypedCode } from '../api'
import type { RedemptionResult } from '../types'

// ============================================================================
// Redemption Hook
// ============================================================================

export function getRedemptionSuccessMessage(result: RedemptionResult): string {
  if (result.type === 'subscription') {
    return i18next.t('Subscription activated: {{plan}}', {
      plan: result.plan_title,
    })
  }
  return i18next.t('Redemption successful! Added: {{quota}}', {
    quota: formatQuota(result.quota),
  })
}

export function useRedemption() {
  const [redeeming, setRedeeming] = useState(false)

  const redeemCode = useCallback(async (code: string): Promise<boolean> => {
    if (!code || code.trim() === '') {
      toast.error(i18next.t('Please enter a redemption code'))
      return false
    }

    try {
      setRedeeming(true)
      const response = await redeemTypedCode({ key: code })

      if (response.success && response.data) {
        toast.success(getRedemptionSuccessMessage(response.data))
        await getSelf()
        return true
      }

      toast.error(response.message || i18next.t('Redemption failed'))
      return false
    } catch {
      toast.error(i18next.t('Redemption failed'))
      return false
    } finally {
      setRedeeming(false)
    }
  }, [])

  return {
    redeeming,
    redeemCode,
  }
}
