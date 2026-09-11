import i18next from 'i18next'
import { useState, useEffect, useCallback } from 'react'
import { toast } from 'sonner'

import {
  type AffiliateSummary,
  convertAffiliateCashback,
  createAffiliateWithdrawal,
  getAffiliateSummary,
} from '@/features/affiliates'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { getSelf } from '@/lib/api'

import { getAffiliateCode, transferAffiliateQuota } from '../api'
import { generateAffiliateLink } from '../lib'

// ============================================================================
// Affiliate Hook
// ============================================================================

export function useAffiliate() {
  const [affiliateCode, setAffiliateCode] = useState<string>('')
  const [affiliateLink, setAffiliateLink] = useState<string>('')
  const [summary, setSummary] = useState<AffiliateSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [transferring, setTransferring] = useState(false)
  const [converting, setConverting] = useState(false)
  const [withdrawing, setWithdrawing] = useState(false)
  const { copyToClipboard } = useCopyToClipboard()

  const fetchAffiliateData = useCallback(async () => {
    try {
      setLoading(true)
      const [codeResponse, summaryResponse] = await Promise.all([
        getAffiliateCode(),
        getAffiliateSummary(),
      ])

      if (codeResponse.success && codeResponse.data) {
        setAffiliateCode(codeResponse.data)
        const link = generateAffiliateLink(codeResponse.data)
        setAffiliateLink(link)
      }
      if (summaryResponse.success && summaryResponse.data) {
        setSummary(summaryResponse.data)
      }
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to fetch affiliate data:', error)
    } finally {
      setLoading(false)
    }
  }, [])

  // Copy affiliate link
  const copyAffiliateLink = useCallback(() => {
    copyToClipboard(affiliateLink)
  }, [affiliateLink, copyToClipboard])

  // Transfer affiliate quota to balance
  const transferQuota = useCallback(async (quota: number): Promise<boolean> => {
    try {
      setTransferring(true)
      const response = await transferAffiliateQuota({ quota })

      if (response.success) {
        toast.success(response.message || i18next.t('Transfer successful'))
        await getSelf()
        return true
      }

      toast.error(response.message || i18next.t('Transfer failed'))
      return false
    } catch {
      toast.error(i18next.t('Transfer failed'))
      return false
    } finally {
      setTransferring(false)
    }
  }, [])

  const convertCashback = useCallback(
    async (amountQuota: number): Promise<boolean> => {
      try {
        setConverting(true)
        const response = await convertAffiliateCashback(amountQuota)
        if (!response.success) {
          toast.error(response.message || i18next.t('Conversion failed'))
          return false
        }
        toast.success(i18next.t('Cashback converted to balance'))
        await Promise.all([getSelf(), fetchAffiliateData()])
        return true
      } catch {
        toast.error(i18next.t('Conversion failed'))
        return false
      } finally {
        setConverting(false)
      }
    },
    [fetchAffiliateData]
  )

  const requestWithdrawal = useCallback(
    async (amountQuota: number, note: string): Promise<boolean> => {
      try {
        setWithdrawing(true)
        const response = await createAffiliateWithdrawal({
          amount_quota: amountQuota,
          note,
        })
        if (!response.success) {
          toast.error(
            response.message || i18next.t('Withdrawal request failed')
          )
          return false
        }
        toast.success(i18next.t('Withdrawal request submitted'))
        await fetchAffiliateData()
        return true
      } catch {
        toast.error(i18next.t('Withdrawal request failed'))
        return false
      } finally {
        setWithdrawing(false)
      }
    },
    [fetchAffiliateData]
  )

  useEffect(() => {
    fetchAffiliateData()
  }, [fetchAffiliateData])

  return {
    affiliateCode,
    affiliateLink,
    summary,
    loading,
    transferring,
    converting,
    withdrawing,
    copyAffiliateLink,
    transferQuota,
    convertCashback,
    requestWithdrawal,
    refetch: fetchAffiliateData,
  }
}
