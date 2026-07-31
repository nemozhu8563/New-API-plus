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
import { Share2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import type { AffiliateSummary } from '@/features/affiliates'
import { formatQuota } from '@/lib/format'

import type { UserWalletData } from '../types'

interface AffiliateRewardsCardProps {
  user: UserWalletData | null
  summary: AffiliateSummary | null
  affiliateLink: string
  onViewDetails: () => void
  onConvertCashback: () => void
  onRequestWithdrawal: () => void
  onTransferLegacyRewards: () => void
  complianceConfirmed?: boolean
  loading?: boolean
}

export function AffiliateRewardsCard({
  user,
  summary,
  affiliateLink,
  onViewDetails,
  onConvertCashback,
  onRequestWithdrawal,
  onTransferLegacyRewards,
  complianceConfirmed = true,
  loading,
}: AffiliateRewardsCardProps) {
  const { t } = useTranslation()
  if (loading) {
    return (
      <Card data-card-hover='false' className='bg-muted/20 py-0'>
        <CardContent className='grid gap-4 p-3 sm:p-4 lg:grid-cols-[minmax(220px,1fr)_minmax(220px,0.72fr)_minmax(320px,1.15fr)] lg:items-center'>
          <div>
            <Skeleton className='h-5 w-32' />
            <Skeleton className='mt-2 h-4 w-48' />
          </div>
          <Skeleton className='h-14 rounded-lg' />
          <Skeleton className='h-10 rounded-lg' />
        </CardContent>
      </Card>
    )
  }

  const hasAgentAccount = summary?.is_agent === true
  const isActiveAgent = hasAgentAccount && summary?.enabled === true
  const hasCashback = (summary?.available_quota ?? 0) > 0
  const hasLegacyRewards = (user?.aff_quota ?? 0) > 0
  const metrics = isActiveAgent
    ? [
        [t('Available cashback'), formatQuota(summary?.available_quota ?? 0)],
        [
          t('Pending withdrawal'),
          formatQuota(summary?.pending_withdrawal_quota ?? 0),
        ],
        [t('Converted'), formatQuota(summary?.converted_quota ?? 0)],
        [t('Withdrawn'), formatQuota(summary?.withdrawn_quota ?? 0)],
      ]
    : [
        [t('Invited accounts'), String(summary?.invitee_count ?? 0)],
        [t('Redemptions'), String(summary?.redemption_count ?? 0)],
        [t('Redeemed face value'), formatQuota(summary?.redeemed_quota ?? 0)],
        [
          t('First redemption rewards'),
          formatQuota(summary?.ordinary_reward_quota ?? 0),
        ],
      ]

  return (
    <Card data-card-hover='false' className='bg-muted/20 py-0'>
      <CardContent className='grid gap-3 p-3 sm:gap-4 sm:p-4 xl:grid-cols-[minmax(220px,0.8fr)_minmax(360px,1.2fr)_minmax(320px,1fr)] xl:items-center'>
        <div className='flex min-w-0 items-center gap-2.5'>
          <IconBadge tone='chart-3'>
            <Share2 />
          </IconBadge>
          <div className='min-w-0'>
            <h3 className='truncate text-sm font-semibold'>
              {t('Referral Program')}
            </h3>
            <div className='mt-1 flex flex-wrap items-center gap-1.5'>
              <Badge variant={isActiveAgent ? 'default' : 'secondary'}>
                {isActiveAgent ? t('Selected agent') : t('Standard inviter')}
              </Badge>
              {isActiveAgent ? (
                <Badge variant='outline'>
                  {t('{{rate}}% permanent cashback', {
                    rate: ((summary?.commission_rate_bps ?? 0) / 100).toFixed(
                      2
                    ),
                  })}
                </Badge>
              ) : (
                <span className='text-muted-foreground text-xs'>
                  {t('5% site balance on each invitee’s first redemption')}
                </span>
              )}
            </div>
          </div>
        </div>

        <div className='grid grid-cols-2 gap-2 text-center sm:grid-cols-4'>
          {metrics.map(([label, value]) => (
            <div key={label}>
              <div className='text-muted-foreground truncate text-[10px] font-medium tracking-wider uppercase'>
                {label}
              </div>
              <div className='mt-0.5 truncate text-sm font-semibold tabular-nums'>
                {value}
              </div>
            </div>
          ))}
        </div>

        <div className='space-y-2'>
          <div className='flex items-center gap-2'>
            <Input
              value={affiliateLink}
              readOnly
              className='border-muted bg-background/70 h-9 min-w-0 flex-1 font-mono text-xs'
            />
            <CopyButton
              value={affiliateLink}
              variant='outline'
              className='bg-background size-9 shrink-0'
              iconClassName='size-4'
              tooltip={t('Copy referral link')}
              aria-label={t('Copy referral link')}
            />
          </div>
          <div className='flex flex-wrap gap-2'>
            <Button variant='outline' size='sm' onClick={onViewDetails}>
              {t('View records')}
            </Button>
            {hasAgentAccount && hasCashback ? (
              <Button
                size='sm'
                onClick={onConvertCashback}
                disabled={!complianceConfirmed}
              >
                {t('Convert to balance')}
              </Button>
            ) : null}
            {hasAgentAccount &&
            hasCashback &&
            summary?.cash_withdrawal_enabled === true ? (
              <Button
                variant='secondary'
                size='sm'
                onClick={onRequestWithdrawal}
                disabled={!complianceConfirmed}
              >
                {t('Request withdrawal')}
              </Button>
            ) : null}
            {hasLegacyRewards ? (
              <Button
                variant='ghost'
                size='sm'
                onClick={onTransferLegacyRewards}
                disabled={!complianceConfirmed}
              >
                {t('Transfer legacy rewards')}
              </Button>
            ) : null}
          </div>
        </div>
        {!complianceConfirmed ? (
          <p className='text-muted-foreground text-xs xl:col-span-3'>
            {t(
              'Referral reward transfer is disabled until the administrator confirms compliance terms.'
            )}
          </p>
        ) : null}
      </CardContent>
    </Card>
  )
}
