import { useQuery } from '@tanstack/react-query'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  type AffiliateApiResponse,
  type AffiliateCommissionRecord,
  type AffiliateConversionRecord,
  type AffiliateInviteeRecord,
  type AffiliatePage,
  type AffiliateRedemptionRecord,
  type AffiliateSummary,
  type AffiliateWithdrawalRecord,
  getAffiliateCommissions,
  getAffiliateConversions,
  getAffiliateInvitees,
  getAffiliateRedemptions,
  getAffiliateWithdrawals,
} from '@/features/affiliates'
import { formatQuota, formatTimestampToDate } from '@/lib/format'

type DetailTab =
  | 'invitees'
  | 'redemptions'
  | 'rewards'
  | 'conversions'
  | 'withdrawals'

type DetailRecord =
  | AffiliateInviteeRecord
  | AffiliateRedemptionRecord
  | AffiliateCommissionRecord
  | AffiliateConversionRecord
  | AffiliateWithdrawalRecord

interface AffiliateDetailsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  summary: AffiliateSummary | null
}

const PAGE_SIZE = 10
const RECORD_SKELETON_KEYS = [
  'affiliate-record-skeleton-1',
  'affiliate-record-skeleton-2',
  'affiliate-record-skeleton-3',
  'affiliate-record-skeleton-4',
] as const

export function AffiliateDetailsDialog({
  open,
  onOpenChange,
  summary,
}: AffiliateDetailsDialogProps) {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<DetailTab>('invitees')
  const [page, setPage] = useState(1)

  const recordsQuery = useQuery<AffiliatePage<DetailRecord>, Error>({
    queryKey: ['affiliate-details', activeTab, page, PAGE_SIZE],
    queryFn: async () => {
      let response: AffiliateApiResponse<AffiliatePage<DetailRecord>>
      switch (activeTab) {
        case 'invitees':
          response = await getAffiliateInvitees(page, PAGE_SIZE)
          break
        case 'redemptions':
          response = await getAffiliateRedemptions(page, PAGE_SIZE)
          break
        case 'rewards':
          response = await getAffiliateCommissions(page, PAGE_SIZE)
          break
        case 'conversions':
          response = await getAffiliateConversions(page, PAGE_SIZE)
          break
        case 'withdrawals':
          response = await getAffiliateWithdrawals(page, PAGE_SIZE)
          break
      }

      if (!response.success || !response.data) {
        throw new Error(response.message)
      }
      return response.data
    },
    enabled: open,
    staleTime: 0,
  })

  const records = recordsQuery.data?.items ?? []
  const total = recordsQuery.data?.total ?? 0
  const loading = recordsQuery.isPending
  const error = recordsQuery.isError
    ? recordsQuery.error.message || t('Failed to load affiliate records')
    : ''

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) {
      setActiveTab('invitees')
      setPage(1)
    }
    onOpenChange(nextOpen)
  }

  const handleTabChange = (value: string) => {
    setActiveTab(value as DetailTab)
    setPage(1)
  }

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  const rewardTypeLabel = (rewardType: string) => {
    if (rewardType === 'ordinary_first') {
      return t('First redemption reward')
    }
    if (rewardType === 'selected_agent') {
      return t('Selected agent cashback')
    }
    return t('No reward')
  }

  const destinationLabel = (destination: string) => {
    if (destination === 'site_balance') {
      return t('Site balance')
    }
    if (destination === 'cashback') {
      return t('Available cashback')
    }
    return '-'
  }

  const withdrawalStatusLabel = (status: string) => {
    if (status === 'pending') {
      return t('Pending payment')
    }
    if (status === 'paid') {
      return t('Paid')
    }
    return t('Rejected')
  }

  const withdrawalBadgeVariant = (
    status: string
  ): 'destructive' | 'warning' | 'secondary' => {
    if (status === 'rejected') {
      return 'destructive'
    }
    if (status === 'pending') {
      return 'warning'
    }
    return 'secondary'
  }

  const renderRecords = () => {
    if (loading) {
      return (
        <div className='space-y-3'>
          {RECORD_SKELETON_KEYS.map((key) => (
            <Skeleton key={key} className='h-24 w-full rounded-lg' />
          ))}
        </div>
      )
    }
    if (error) {
      return (
        <div className='text-destructive flex min-h-40 items-center justify-center text-sm'>
          {error}
        </div>
      )
    }
    if (records.length === 0) {
      return (
        <div className='text-muted-foreground flex min-h-40 items-center justify-center text-sm'>
          {t('No affiliate records found')}
        </div>
      )
    }

    if (activeTab === 'invitees') {
      return (records as AffiliateInviteeRecord[]).map((record) => (
        <div key={record.user_id} className='rounded-lg border p-3'>
          <div className='flex items-start justify-between gap-3'>
            <div>
              <div className='font-medium'>{record.username}</div>
              {record.display_name ? (
                <div className='text-muted-foreground text-xs'>
                  {record.display_name}
                </div>
              ) : null}
            </div>
            <Badge variant='outline'>
              {t('{{count}} redemptions', {
                count: record.redemption_count,
              })}
            </Badge>
          </div>
          <div className='mt-3 grid grid-cols-2 gap-3 text-xs sm:grid-cols-3'>
            <RecordValue
              label={t('Redeemed face value')}
              value={formatQuota(record.redeemed_quota)}
            />
            <RecordValue
              label={t('Reward earned')}
              value={formatQuota(record.reward_quota)}
            />
            <RecordValue
              label={t('Joined at')}
              value={formatTimestampToDate(record.created_at)}
            />
          </div>
        </div>
      ))
    }

    if (activeTab === 'redemptions') {
      return (records as AffiliateRedemptionRecord[]).map((record) => (
        <div key={record.redemption_id} className='rounded-lg border p-3'>
          <div className='flex items-start justify-between gap-3'>
            <div>
              <div className='font-medium'>{record.invitee_username}</div>
              <div className='text-muted-foreground text-xs'>
                {formatTimestampToDate(record.redeemed_at)}
              </div>
            </div>
            <div className='text-right'>
              <div className='font-semibold tabular-nums'>
                {formatQuota(record.source_quota)}
              </div>
              <div className='text-muted-foreground text-xs'>
                {t('Face value')}
              </div>
            </div>
          </div>
          <div className='mt-3 grid grid-cols-2 gap-3 text-xs sm:grid-cols-3'>
            <RecordValue
              label={t('Reward type')}
              value={rewardTypeLabel(record.reward_type)}
            />
            <RecordValue
              label={t('Reward rate')}
              value={
                record.rate_bps > 0
                  ? `${(record.rate_bps / 100).toFixed(2)}%`
                  : '-'
              }
            />
            <RecordValue
              label={t('Reward amount')}
              value={formatQuota(record.reward_quota)}
            />
          </div>
        </div>
      ))
    }

    if (activeTab === 'rewards') {
      return (records as AffiliateCommissionRecord[]).map((record) => (
        <div key={record.id} className='rounded-lg border p-3'>
          <div className='flex items-start justify-between gap-3'>
            <div>
              <div className='font-medium'>
                {rewardTypeLabel(record.reward_type)}
              </div>
              <div className='text-muted-foreground text-xs'>
                {record.invitee_username} ·{' '}
                {formatTimestampToDate(record.created_at)}
              </div>
            </div>
            <div className='font-semibold tabular-nums'>
              +{formatQuota(record.commission_quota)}
            </div>
          </div>
          <div className='mt-3 grid grid-cols-2 gap-3 text-xs sm:grid-cols-3'>
            <RecordValue
              label={t('Redeemed face value')}
              value={formatQuota(record.source_quota)}
            />
            <RecordValue
              label={t('Reward rate')}
              value={`${(record.rate_bps / 100).toFixed(2)}%`}
            />
            <RecordValue
              label={t('Destination')}
              value={destinationLabel(record.destination)}
            />
          </div>
        </div>
      ))
    }

    if (activeTab === 'conversions') {
      return (records as AffiliateConversionRecord[]).map((record) => (
        <div
          key={record.id}
          className='flex items-center justify-between gap-3 rounded-lg border p-3'
        >
          <div>
            <div className='font-medium'>{t('Converted to site balance')}</div>
            <div className='text-muted-foreground text-xs'>
              {formatTimestampToDate(record.created_at)}
            </div>
          </div>
          <div className='font-semibold tabular-nums'>
            {formatQuota(record.amount_quota)}
          </div>
        </div>
      ))
    }

    return (records as AffiliateWithdrawalRecord[]).map((record) => (
      <div key={record.id} className='rounded-lg border p-3'>
        <div className='flex items-start justify-between gap-3'>
          <div>
            <div className='font-medium'>
              {t('Withdrawal request #{{id}}', { id: record.id })}
            </div>
            <div className='text-muted-foreground text-xs'>
              {formatTimestampToDate(record.created_at)}
            </div>
          </div>
          <div className='text-right'>
            <div className='font-semibold tabular-nums'>
              {formatQuota(record.amount_quota)}
            </div>
            <Badge variant={withdrawalBadgeVariant(record.status)}>
              {withdrawalStatusLabel(record.status)}
            </Badge>
          </div>
        </div>
        {record.applicant_note || record.admin_note ? (
          <div className='bg-muted/40 mt-3 space-y-1 rounded-md p-2 text-xs'>
            {record.applicant_note ? (
              <div>
                {t('Applicant note')}: {record.applicant_note}
              </div>
            ) : null}
            {record.admin_note ? (
              <div>
                {t('Admin note')}: {record.admin_note}
              </div>
            ) : null}
          </div>
        ) : null}
      </div>
    ))
  }

  const tabs: Array<{ value: DetailTab; label: string; agentOnly?: boolean }> =
    [
      { value: 'invitees', label: t('Invited accounts') },
      { value: 'redemptions', label: t('Redemptions') },
      { value: 'rewards', label: t('Reward records') },
      { value: 'conversions', label: t('Conversions'), agentOnly: true },
      { value: 'withdrawals', label: t('Withdrawals'), agentOnly: true },
    ]

  return (
    <Dialog
      open={open}
      onOpenChange={handleOpenChange}
      title={t('Affiliate details')}
      description={t(
        'View invited accounts, redeemed face value, rewards, conversions, and withdrawals.'
      )}
      contentClassName='max-sm:w-[calc(100vw-1rem)] sm:max-w-4xl'
      contentHeight='min(66vh, 640px)'
      bodyClassName='space-y-4'
    >
      <Tabs value={activeTab} onValueChange={handleTabChange}>
        <TabsList className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'>
          {tabs
            .filter((tab) => !tab.agentOnly || summary?.is_agent)
            .map((tab) => (
              <TabsTrigger key={tab.value} value={tab.value}>
                {tab.label}
              </TabsTrigger>
            ))}
        </TabsList>
      </Tabs>

      <div className='space-y-3'>{renderRecords()}</div>

      {!loading && !error && total > 0 ? (
        <div className='flex items-center justify-between border-t pt-3'>
          <div className='text-muted-foreground text-xs'>
            {t('Showing')} {(page - 1) * PAGE_SIZE + 1}-
            {Math.min(page * PAGE_SIZE, total)} {t('of')} {total}
          </div>
          <div className='flex items-center gap-2'>
            <Button
              variant='outline'
              size='icon-sm'
              onClick={() => setPage((current) => Math.max(1, current - 1))}
              disabled={page <= 1}
              aria-label={t('Previous page')}
            >
              <ChevronLeft />
            </Button>
            <span className='text-muted-foreground text-xs tabular-nums'>
              {page}/{totalPages}
            </span>
            <Button
              variant='outline'
              size='icon-sm'
              onClick={() =>
                setPage((current) => Math.min(totalPages, current + 1))
              }
              disabled={page >= totalPages}
              aria-label={t('Next page')}
            >
              <ChevronRight />
            </Button>
          </div>
        </div>
      ) : null}
    </Dialog>
  )
}

function RecordValue({ label, value }: { label: string; value: string }) {
  return (
    <div className='min-w-0'>
      <div className='text-muted-foreground truncate'>{label}</div>
      <div className='mt-0.5 truncate font-medium tabular-nums'>{value}</div>
    </div>
  )
}
