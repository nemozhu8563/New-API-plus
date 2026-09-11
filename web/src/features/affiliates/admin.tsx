import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronLeft, ChevronRight, Loader2, Plus, Search } from 'lucide-react'
import { useState, type FormEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { formatQuota, formatTimestampToDate } from '@/lib/format'

import {
  adminListAffiliateAgents,
  adminListAffiliateCommissions,
  adminListAffiliateConversions,
  adminListAffiliateInvitations,
  adminListAffiliateRedemptions,
  adminListAffiliateWithdrawals,
  adminReviewAffiliateWithdrawal,
  adminUpdateAffiliateAgent,
} from './api'
import type {
  AffiliateAgentRecord,
  AffiliateCommissionRecord,
  AffiliateConversionRecord,
  AffiliateInvitationRecord,
  AffiliateRedemptionRecord,
  AffiliateWithdrawalRecord,
  AffiliateWithdrawalStatus,
} from './types'

type AffiliateAdminTab =
  | 'agents'
  | 'invitations'
  | 'redemptions'
  | 'rewards'
  | 'conversions'
  | 'withdrawals'

type AffiliateAdminRecord =
  | AffiliateAgentRecord
  | AffiliateInvitationRecord
  | AffiliateRedemptionRecord
  | AffiliateCommissionRecord
  | AffiliateConversionRecord
  | AffiliateWithdrawalRecord

interface AgentFormState {
  userId: number
  enabled: boolean
  ratePercent: number
  cashWithdrawalEnabled: boolean
}

interface WithdrawalReviewState {
  record: AffiliateWithdrawalRecord
  decision: 'pay' | 'reject'
}

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100]
const TABLE_SKELETON_KEYS = [
  'affiliate-table-skeleton-1',
  'affiliate-table-skeleton-2',
  'affiliate-table-skeleton-3',
  'affiliate-table-skeleton-4',
  'affiliate-table-skeleton-5',
  'affiliate-table-skeleton-6',
] as const

export function AffiliateAdmin() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [activeTab, setActiveTab] = useState<AffiliateAdminTab>('agents')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [searchInput, setSearchInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [withdrawalStatus, setWithdrawalStatus] = useState<
    AffiliateWithdrawalStatus | 'all'
  >('all')
  const [agentDialogOpen, setAgentDialogOpen] = useState(false)
  const [editingAgentUserId, setEditingAgentUserId] = useState<number | null>(
    null
  )
  const [agentForm, setAgentForm] = useState<AgentFormState>({
    userId: 0,
    enabled: true,
    ratePercent: 5,
    cashWithdrawalEnabled: false,
  })
  const [savingAgent, setSavingAgent] = useState(false)
  const [reviewState, setReviewState] = useState<WithdrawalReviewState | null>(
    null
  )
  const [reviewNote, setReviewNote] = useState('')
  const [reviewing, setReviewing] = useState(false)

  const supportsKeyword = !['conversions', 'withdrawals'].includes(activeTab)

  const recordsQuery = useQuery({
    queryKey: [
      'admin-affiliates',
      activeTab,
      page,
      pageSize,
      keyword,
      withdrawalStatus,
    ],
    queryFn: async (): Promise<{
      items: AffiliateAdminRecord[]
      total: number
    }> => {
      if (activeTab === 'agents') {
        const response = await adminListAffiliateAgents(page, pageSize, keyword)
        if (!response.success) {
          throw new Error(response.message || t('Failed to load agents'))
        }
        return {
          items: response.data?.items ?? [],
          total: response.data?.total ?? 0,
        }
      }
      if (activeTab === 'invitations') {
        const response = await adminListAffiliateInvitations(
          page,
          pageSize,
          keyword
        )
        if (!response.success) {
          throw new Error(response.message || t('Failed to load invitations'))
        }
        return {
          items: response.data?.items ?? [],
          total: response.data?.total ?? 0,
        }
      }
      if (activeTab === 'redemptions') {
        const response = await adminListAffiliateRedemptions(
          page,
          pageSize,
          keyword
        )
        if (!response.success) {
          throw new Error(response.message || t('Failed to load redemptions'))
        }
        return {
          items: response.data?.items ?? [],
          total: response.data?.total ?? 0,
        }
      }
      if (activeTab === 'rewards') {
        const response = await adminListAffiliateCommissions(
          page,
          pageSize,
          keyword
        )
        if (!response.success) {
          throw new Error(response.message || t('Failed to load rewards'))
        }
        return {
          items: response.data?.items ?? [],
          total: response.data?.total ?? 0,
        }
      }
      if (activeTab === 'conversions') {
        const response = await adminListAffiliateConversions(page, pageSize)
        if (!response.success) {
          throw new Error(response.message || t('Failed to load conversions'))
        }
        return {
          items: response.data?.items ?? [],
          total: response.data?.total ?? 0,
        }
      }

      const response = await adminListAffiliateWithdrawals(
        page,
        pageSize,
        withdrawalStatus === 'all' ? undefined : withdrawalStatus
      )
      if (!response.success) {
        throw new Error(response.message || t('Failed to load withdrawals'))
      }
      return {
        items: response.data?.items ?? [],
        total: response.data?.total ?? 0,
      }
    },
  })

  const records = recordsQuery.data?.items ?? []
  const total = recordsQuery.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  const handleTabChange = (value: string) => {
    setActiveTab(value as AffiliateAdminTab)
    setPage(1)
    setKeyword('')
    setSearchInput('')
  }

  const handleSearch = (event: FormEvent) => {
    event.preventDefault()
    setPage(1)
    setKeyword(searchInput.trim())
  }

  const openCreateAgent = () => {
    setEditingAgentUserId(null)
    setAgentForm({
      userId: 0,
      enabled: true,
      ratePercent: 5,
      cashWithdrawalEnabled: false,
    })
    setAgentDialogOpen(true)
  }

  const openEditAgent = (agent: AffiliateAgentRecord) => {
    setEditingAgentUserId(agent.user_id)
    setAgentForm({
      userId: agent.user_id,
      enabled: agent.enabled,
      ratePercent: agent.commission_rate_bps / 100,
      cashWithdrawalEnabled: agent.cash_withdrawal_enabled,
    })
    setAgentDialogOpen(true)
  }

  const saveAgent = async () => {
    const rateBps = Math.round(agentForm.ratePercent * 100)
    if (
      !Number.isInteger(agentForm.userId) ||
      agentForm.userId <= 0 ||
      !Number.isFinite(agentForm.ratePercent) ||
      rateBps < 500 ||
      rateBps > 1000
    ) {
      toast.error(t('Enter a valid user ID and a cashback rate from 5% to 10%'))
      return
    }

    try {
      setSavingAgent(true)
      const response = await adminUpdateAffiliateAgent(agentForm.userId, {
        enabled: agentForm.enabled,
        commission_rate_bps: rateBps,
        cash_withdrawal_enabled: agentForm.cashWithdrawalEnabled,
      })
      if (!response.success) {
        toast.error(response.message || t('Failed to save agent settings'))
        return
      }
      toast.success(t('Agent settings saved'))
      setAgentDialogOpen(false)
      await queryClient.invalidateQueries({ queryKey: ['admin-affiliates'] })
    } catch {
      toast.error(t('Failed to save agent settings'))
    } finally {
      setSavingAgent(false)
    }
  }

  const openReview = (
    record: AffiliateWithdrawalRecord,
    decision: 'pay' | 'reject'
  ) => {
    setReviewNote('')
    setReviewState({ record, decision })
  }

  const submitReview = async () => {
    if (!reviewState || reviewNote.length > 500) return
    try {
      setReviewing(true)
      const response = await adminReviewAffiliateWithdrawal(
        reviewState.record.id,
        reviewState.decision,
        reviewNote
      )
      if (!response.success) {
        toast.error(response.message || t('Failed to review withdrawal'))
        return
      }
      toast.success(
        reviewState.decision === 'pay'
          ? t('Withdrawal marked as paid')
          : t('Withdrawal rejected and cashback returned')
      )
      setReviewState(null)
      await queryClient.invalidateQueries({ queryKey: ['admin-affiliates'] })
    } catch {
      toast.error(t('Failed to review withdrawal'))
    } finally {
      setReviewing(false)
    }
  }

  const tabs: Array<{ value: AffiliateAdminTab; label: string }> = [
    { value: 'agents', label: t('Selected agents') },
    { value: 'invitations', label: t('Invitation relationships') },
    { value: 'redemptions', label: t('Redemptions') },
    { value: 'rewards', label: t('Reward records') },
    { value: 'conversions', label: t('Conversions') },
    { value: 'withdrawals', label: t('Withdrawals') },
  ]

  let recordsContent: ReactNode
  if (recordsQuery.isLoading) {
    recordsContent = (
      <div className='space-y-3 p-4'>
        {TABLE_SKELETON_KEYS.map((key) => (
          <Skeleton key={key} className='h-12 w-full' />
        ))}
      </div>
    )
  } else if (recordsQuery.error) {
    recordsContent = (
      <div className='text-destructive flex min-h-80 items-center justify-center p-6 text-sm'>
        {recordsQuery.error.message}
      </div>
    )
  } else if (records.length === 0) {
    recordsContent = (
      <div className='text-muted-foreground flex min-h-80 items-center justify-center p-6 text-sm'>
        {t('No affiliate records found')}
      </div>
    )
  } else {
    recordsContent = (
      <AffiliateAdminTable
        activeTab={activeTab}
        records={records}
        onEditAgent={openEditAgent}
        onReviewWithdrawal={openReview}
      />
    )
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Affiliate management')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button size='sm' onClick={openCreateAgent}>
            <Plus />
            {t('Configure agent')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex min-h-full flex-col gap-4'>
            <Tabs value={activeTab} onValueChange={handleTabChange}>
              <TabsList className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'>
                {tabs.map((tab) => (
                  <TabsTrigger key={tab.value} value={tab.value}>
                    {tab.label}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>

            <Card className='min-h-0 flex-1 py-0'>
              <CardContent className='flex min-h-0 flex-1 flex-col gap-3 p-3 sm:p-4'>
                <div className='flex flex-wrap items-center gap-2'>
                  {supportsKeyword ? (
                    <form
                      onSubmit={handleSearch}
                      className='flex min-w-64 flex-1 items-center gap-2'
                    >
                      <div className='relative flex-1'>
                        <Search className='text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2' />
                        <Input
                          value={searchInput}
                          onChange={(event) =>
                            setSearchInput(event.target.value)
                          }
                          placeholder={t('Search by inviter or invitee')}
                          className='pl-9'
                        />
                      </div>
                      <Button type='submit' variant='outline'>
                        {t('Search')}
                      </Button>
                    </form>
                  ) : null}

                  {activeTab === 'withdrawals' ? (
                    <Select
                      value={withdrawalStatus}
                      onValueChange={(value) => {
                        setWithdrawalStatus(
                          (value ?? 'all') as AffiliateWithdrawalStatus | 'all'
                        )
                        setPage(1)
                      }}
                    >
                      <SelectTrigger className='w-40'>
                        <SelectValue placeholder={t('All statuses')} />
                      </SelectTrigger>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          <SelectItem value='all'>
                            {t('All statuses')}
                          </SelectItem>
                          <SelectItem value='pending'>
                            {t('Pending payment')}
                          </SelectItem>
                          <SelectItem value='paid'>{t('Paid')}</SelectItem>
                          <SelectItem value='rejected'>
                            {t('Rejected')}
                          </SelectItem>
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  ) : null}

                  <Select
                    value={String(pageSize)}
                    onValueChange={(value) => {
                      if (value === null) return
                      setPageSize(Number(value))
                      setPage(1)
                    }}
                  >
                    <SelectTrigger className='w-28'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {PAGE_SIZE_OPTIONS.map((option) => (
                          <SelectItem key={option} value={String(option)}>
                            {t('{{count}} / page', { count: option })}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                </div>

                <div className='min-h-80 flex-1 overflow-auto rounded-lg border'>
                  {recordsContent}
                </div>

                <div className='flex flex-wrap items-center justify-between gap-2'>
                  <div className='text-muted-foreground text-xs'>
                    {total > 0
                      ? t('Showing {{start}}-{{end}} of {{total}}', {
                          start: (page - 1) * pageSize + 1,
                          end: Math.min(page * pageSize, total),
                          total,
                        })
                      : t('No records')}
                  </div>
                  <div className='flex items-center gap-2'>
                    <Button
                      variant='outline'
                      size='icon-sm'
                      onClick={() =>
                        setPage((current) => Math.max(1, current - 1))
                      }
                      disabled={page <= 1 || recordsQuery.isFetching}
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
                      disabled={page >= totalPages || recordsQuery.isFetching}
                      aria-label={t('Next page')}
                    >
                      <ChevronRight />
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <Dialog
        open={agentDialogOpen}
        onOpenChange={setAgentDialogOpen}
        title={t('Configure selected agent')}
        description={t(
          'Set a permanent cashback rate from 5% to 10% for one account.'
        )}
        contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'
        contentHeight='auto'
        bodyClassName='space-y-5 py-2'
        footer={
          <>
            <Button
              variant='outline'
              onClick={() => setAgentDialogOpen(false)}
              disabled={savingAgent}
            >
              {t('Cancel')}
            </Button>
            <Button onClick={saveAgent} disabled={savingAgent}>
              {savingAgent ? <Loader2 className='animate-spin' /> : null}
              {t('Save')}
            </Button>
          </>
        }
      >
        <div className='space-y-2'>
          <Label htmlFor='affiliate-agent-user-id'>{t('User ID')}</Label>
          <Input
            id='affiliate-agent-user-id'
            type='number'
            min={1}
            step={1}
            value={agentForm.userId || ''}
            onChange={(event) =>
              setAgentForm((current) => ({
                ...current,
                userId: Number(event.target.value),
              }))
            }
            disabled={editingAgentUserId !== null}
          />
        </div>
        <div className='space-y-2'>
          <Label htmlFor='affiliate-agent-rate'>{t('Cashback rate')}</Label>
          <div className='relative'>
            <Input
              id='affiliate-agent-rate'
              type='number'
              min={5}
              max={10}
              step={0.01}
              value={agentForm.ratePercent}
              onChange={(event) =>
                setAgentForm((current) => ({
                  ...current,
                  ratePercent: Number(event.target.value),
                }))
              }
              className='pr-8'
            />
            <span className='text-muted-foreground absolute top-1/2 right-3 -translate-y-1/2 text-sm'>
              %
            </span>
          </div>
        </div>
        <div className='flex items-center justify-between gap-4 rounded-lg border p-3'>
          <div>
            <Label htmlFor='affiliate-agent-enabled'>
              {t('Enable selected agent')}
            </Label>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t('Only new redemptions are affected.')}
            </p>
          </div>
          <Switch
            id='affiliate-agent-enabled'
            checked={agentForm.enabled}
            onCheckedChange={(checked) =>
              setAgentForm((current) => ({ ...current, enabled: checked }))
            }
          />
        </div>
        <div className='flex items-center justify-between gap-4 rounded-lg border p-3'>
          <div>
            <Label htmlFor='affiliate-agent-withdrawal-enabled'>
              {t('Allow withdrawal requests')}
            </Label>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t('Payments are completed offline and confirmed by an admin.')}
            </p>
          </div>
          <Switch
            id='affiliate-agent-withdrawal-enabled'
            checked={agentForm.cashWithdrawalEnabled}
            onCheckedChange={(checked) =>
              setAgentForm((current) => ({
                ...current,
                cashWithdrawalEnabled: checked,
              }))
            }
          />
        </div>
      </Dialog>

      <Dialog
        open={reviewState !== null}
        onOpenChange={(open) => !open && setReviewState(null)}
        title={
          reviewState?.decision === 'pay'
            ? t('Confirm offline payment')
            : t('Reject withdrawal')
        }
        description={
          reviewState?.decision === 'pay'
            ? t(
                'Only confirm after the offline transfer has been completed. This action marks the request as paid.'
              )
            : t(
                'Rejecting returns the frozen amount to the agent’s available cashback.'
              )
        }
        contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'
        contentHeight='auto'
        bodyClassName='space-y-4 py-2'
        footer={
          <>
            <Button
              variant='outline'
              onClick={() => setReviewState(null)}
              disabled={reviewing}
            >
              {t('Cancel')}
            </Button>
            <Button
              variant={
                reviewState?.decision === 'reject' ? 'destructive' : 'default'
              }
              onClick={submitReview}
              disabled={reviewing || reviewNote.length > 500}
            >
              {reviewing ? <Loader2 className='animate-spin' /> : null}
              {reviewState?.decision === 'pay'
                ? t('Confirm paid')
                : t('Reject and return')}
            </Button>
          </>
        }
      >
        {reviewState ? (
          <div className='grid grid-cols-2 gap-3 rounded-lg border p-3 text-sm'>
            <div>
              <div className='text-muted-foreground text-xs'>{t('Agent')}</div>
              <div className='font-medium'>
                {reviewState.record.agent_username}
              </div>
            </div>
            <div>
              <div className='text-muted-foreground text-xs'>{t('Amount')}</div>
              <div className='font-semibold'>
                {formatQuota(reviewState.record.amount_quota)}
              </div>
            </div>
          </div>
        ) : null}
        <div className='space-y-2'>
          <Label htmlFor='affiliate-review-note'>{t('Admin note')}</Label>
          <Textarea
            id='affiliate-review-note'
            value={reviewNote}
            maxLength={500}
            onChange={(event) => setReviewNote(event.target.value)}
          />
          <p className='text-muted-foreground text-right text-xs'>
            {reviewNote.length}/500
          </p>
        </div>
      </Dialog>
    </>
  )
}

interface AffiliateAdminTableProps {
  activeTab: AffiliateAdminTab
  records: AffiliateAdminRecord[]
  onEditAgent: (agent: AffiliateAgentRecord) => void
  onReviewWithdrawal: (
    record: AffiliateWithdrawalRecord,
    decision: 'pay' | 'reject'
  ) => void
}

function AffiliateAdminTable({
  activeTab,
  records,
  onEditAgent,
  onReviewWithdrawal,
}: AffiliateAdminTableProps) {
  const { t } = useTranslation()

  const statusBadge = (status: AffiliateWithdrawalStatus) => {
    if (status === 'rejected') {
      return <Badge variant='destructive'>{t('Rejected')}</Badge>
    }
    if (status === 'pending') {
      return <Badge variant='warning'>{t('Pending payment')}</Badge>
    }
    return <Badge variant='secondary'>{t('Paid')}</Badge>
  }

  const rewardTypeLabel = (rewardType: string) => {
    if (rewardType === 'ordinary_first') {
      return t('First redemption reward')
    }
    if (rewardType === 'selected_agent') {
      return t('Selected agent cashback')
    }
    return t('No reward')
  }

  if (activeTab === 'agents') {
    return (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Account')}</TableHead>
            <TableHead>{t('Policy')}</TableHead>
            <TableHead>{t('Available cashback')}</TableHead>
            <TableHead>{t('Pending withdrawal')}</TableHead>
            <TableHead>{t('Lifetime cashback')}</TableHead>
            <TableHead className='text-right'>{t('Actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {(records as AffiliateAgentRecord[]).map((record) => (
            <TableRow key={record.user_id}>
              <TableCell>
                <div className='font-medium'>{record.username}</div>
                <div className='text-muted-foreground text-xs'>
                  ID {record.user_id}
                  {record.display_name ? ` · ${record.display_name}` : ''}
                </div>
              </TableCell>
              <TableCell>
                <div className='flex flex-wrap gap-1'>
                  <Badge variant={record.enabled ? 'default' : 'secondary'}>
                    {record.enabled ? t('Enabled') : t('Disabled')}
                  </Badge>
                  <Badge variant='outline'>
                    {(record.commission_rate_bps / 100).toFixed(2)}%
                  </Badge>
                  {record.cash_withdrawal_enabled ? (
                    <Badge variant='outline'>{t('Withdrawal enabled')}</Badge>
                  ) : null}
                </div>
              </TableCell>
              <TableCell>{formatQuota(record.available_quota)}</TableCell>
              <TableCell>
                {formatQuota(record.pending_withdrawal_quota)}
              </TableCell>
              <TableCell>
                {formatQuota(record.total_commission_quota)}
              </TableCell>
              <TableCell className='text-right'>
                <Button
                  variant='outline'
                  size='sm'
                  onClick={() => onEditAgent(record)}
                >
                  {t('Edit')}
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    )
  }

  if (activeTab === 'invitations') {
    return (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Inviter')}</TableHead>
            <TableHead>{t('Invitee')}</TableHead>
            <TableHead>{t('Joined at')}</TableHead>
            <TableHead>{t('Redemptions')}</TableHead>
            <TableHead>{t('Redeemed face value')}</TableHead>
            <TableHead>{t('Reward earned')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {(records as AffiliateInvitationRecord[]).map((record) => (
            <TableRow
              key={`${record.inviter_user_id}-${record.invitee_user_id}`}
            >
              <TableCell>
                <div>{record.inviter_username}</div>
                <div className='text-muted-foreground text-xs'>
                  ID {record.inviter_user_id}
                </div>
              </TableCell>
              <TableCell>
                <div>{record.invitee_username}</div>
                <div className='text-muted-foreground text-xs'>
                  ID {record.invitee_user_id}
                  {record.invitee_name ? ` · ${record.invitee_name}` : ''}
                </div>
              </TableCell>
              <TableCell>{formatTimestampToDate(record.created_at)}</TableCell>
              <TableCell>{record.redemption_count}</TableCell>
              <TableCell>{formatQuota(record.redeemed_quota)}</TableCell>
              <TableCell>{formatQuota(record.reward_quota)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    )
  }

  if (activeTab === 'redemptions') {
    return (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Inviter')}</TableHead>
            <TableHead>{t('Invitee')}</TableHead>
            <TableHead>{t('Redeemed at')}</TableHead>
            <TableHead>{t('Face value')}</TableHead>
            <TableHead>{t('Reward type')}</TableHead>
            <TableHead>{t('Reward amount')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {(records as AffiliateRedemptionRecord[]).map((record) => (
            <TableRow key={record.redemption_id}>
              <TableCell>{record.inviter_username}</TableCell>
              <TableCell>{record.invitee_username}</TableCell>
              <TableCell>{formatTimestampToDate(record.redeemed_at)}</TableCell>
              <TableCell>{formatQuota(record.source_quota)}</TableCell>
              <TableCell>{rewardTypeLabel(record.reward_type)}</TableCell>
              <TableCell>
                {record.rate_bps > 0
                  ? `${formatQuota(record.reward_quota)} (${(
                      record.rate_bps / 100
                    ).toFixed(2)}%)`
                  : '-'}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    )
  }

  if (activeTab === 'rewards') {
    return (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Inviter')}</TableHead>
            <TableHead>{t('Invitee')}</TableHead>
            <TableHead>{t('Created at')}</TableHead>
            <TableHead>{t('Reward type')}</TableHead>
            <TableHead>{t('Face value')}</TableHead>
            <TableHead>{t('Rate')}</TableHead>
            <TableHead>{t('Reward amount')}</TableHead>
            <TableHead>{t('Destination')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {(records as AffiliateCommissionRecord[]).map((record) => (
            <TableRow key={record.id}>
              <TableCell>{record.inviter_username}</TableCell>
              <TableCell>{record.invitee_username}</TableCell>
              <TableCell>{formatTimestampToDate(record.created_at)}</TableCell>
              <TableCell>
                {record.reward_type === 'ordinary_first'
                  ? t('First redemption reward')
                  : t('Selected agent cashback')}
              </TableCell>
              <TableCell>{formatQuota(record.source_quota)}</TableCell>
              <TableCell>{(record.rate_bps / 100).toFixed(2)}%</TableCell>
              <TableCell>{formatQuota(record.commission_quota)}</TableCell>
              <TableCell>
                {record.destination === 'site_balance'
                  ? t('Site balance')
                  : t('Available cashback')}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    )
  }

  if (activeTab === 'conversions') {
    return (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Agent')}</TableHead>
            <TableHead>{t('Converted amount')}</TableHead>
            <TableHead>{t('Created at')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {(records as AffiliateConversionRecord[]).map((record) => (
            <TableRow key={record.id}>
              <TableCell>
                <div>{record.agent_username}</div>
                <div className='text-muted-foreground text-xs'>
                  ID {record.agent_user_id}
                </div>
              </TableCell>
              <TableCell>{formatQuota(record.amount_quota)}</TableCell>
              <TableCell>{formatTimestampToDate(record.created_at)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    )
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('Agent')}</TableHead>
          <TableHead>{t('Amount')}</TableHead>
          <TableHead>{t('Status')}</TableHead>
          <TableHead>{t('Applicant note')}</TableHead>
          <TableHead>{t('Created at')}</TableHead>
          <TableHead>{t('Admin note')}</TableHead>
          <TableHead className='text-right'>{t('Actions')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {(records as AffiliateWithdrawalRecord[]).map((record) => (
          <TableRow key={record.id}>
            <TableCell>
              <div>{record.agent_username}</div>
              <div className='text-muted-foreground text-xs'>
                ID {record.agent_user_id}
              </div>
            </TableCell>
            <TableCell>{formatQuota(record.amount_quota)}</TableCell>
            <TableCell>{statusBadge(record.status)}</TableCell>
            <TableCell className='max-w-56 truncate'>
              {record.applicant_note || '-'}
            </TableCell>
            <TableCell>{formatTimestampToDate(record.created_at)}</TableCell>
            <TableCell className='max-w-56 truncate'>
              {record.admin_note || '-'}
            </TableCell>
            <TableCell className='text-right'>
              {record.status === 'pending' ? (
                <div className='flex justify-end gap-2'>
                  <Button
                    size='sm'
                    variant='outline'
                    onClick={() => onReviewWithdrawal(record, 'reject')}
                  >
                    {t('Reject')}
                  </Button>
                  <Button
                    size='sm'
                    onClick={() => onReviewWithdrawal(record, 'pay')}
                  >
                    {t('Confirm paid')}
                  </Button>
                </div>
              ) : (
                '-'
              )}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
