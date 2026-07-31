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
import { api } from '@/lib/api'

import type {
  AffiliateAgent,
  AffiliateAgentConfigRequest,
  AffiliateAgentRecord,
  AffiliateApiResponse,
  AffiliateCommissionRecord,
  AffiliateConversionRecord,
  AffiliateInvitationRecord,
  AffiliateInviteeRecord,
  AffiliatePage,
  AffiliateRedemptionRecord,
  AffiliateSummary,
  AffiliateWithdrawalRecord,
  AffiliateWithdrawalRequest,
  AffiliateWithdrawalStatus,
} from './types'

function pageParams(page: number, pageSize: number, keyword?: string) {
  const params = new URLSearchParams({
    p: String(page),
    page_size: String(pageSize),
  })
  if (keyword?.trim()) {
    params.set('keyword', keyword.trim())
  }
  return params
}

export async function getAffiliateSummary(): Promise<
  AffiliateApiResponse<AffiliateSummary>
> {
  const response = await api.get('/api/user/affiliate/summary')
  return response.data
}

export async function getAffiliateInvitees(
  page: number,
  pageSize: number
): Promise<AffiliateApiResponse<AffiliatePage<AffiliateInviteeRecord>>> {
  const response = await api.get(
    `/api/user/affiliate/invitees?${pageParams(page, pageSize)}`
  )
  return response.data
}

export async function getAffiliateRedemptions(
  page: number,
  pageSize: number
): Promise<AffiliateApiResponse<AffiliatePage<AffiliateRedemptionRecord>>> {
  const response = await api.get(
    `/api/user/affiliate/redemptions?${pageParams(page, pageSize)}`
  )
  return response.data
}

export async function getAffiliateCommissions(
  page: number,
  pageSize: number
): Promise<AffiliateApiResponse<AffiliatePage<AffiliateCommissionRecord>>> {
  const response = await api.get(
    `/api/user/affiliate/commissions?${pageParams(page, pageSize)}`
  )
  return response.data
}

export async function getAffiliateConversions(
  page: number,
  pageSize: number
): Promise<AffiliateApiResponse<AffiliatePage<AffiliateConversionRecord>>> {
  const response = await api.get(
    `/api/user/affiliate/conversions?${pageParams(page, pageSize)}`
  )
  return response.data
}

export async function getAffiliateWithdrawals(
  page: number,
  pageSize: number
): Promise<AffiliateApiResponse<AffiliatePage<AffiliateWithdrawalRecord>>> {
  const response = await api.get(
    `/api/user/affiliate/withdrawals?${pageParams(page, pageSize)}`
  )
  return response.data
}

export async function convertAffiliateCashback(
  amountQuota: number
): Promise<AffiliateApiResponse<AffiliateConversionRecord>> {
  const response = await api.post('/api/user/affiliate/convert', {
    amount_quota: amountQuota,
  })
  return response.data
}

export async function createAffiliateWithdrawal(
  request: AffiliateWithdrawalRequest
): Promise<AffiliateApiResponse<AffiliateWithdrawalRecord>> {
  const response = await api.post('/api/user/affiliate/withdrawals', request)
  return response.data
}

export async function adminGetAffiliateAgent(
  userId: number
): Promise<AffiliateApiResponse<AffiliateAgentRecord>> {
  const response = await api.get(`/api/affiliate/agents/${userId}`)
  return response.data
}

export async function adminUpdateAffiliateAgent(
  userId: number,
  request: AffiliateAgentConfigRequest
): Promise<AffiliateApiResponse<AffiliateAgent>> {
  const response = await api.put(`/api/affiliate/agents/${userId}`, request)
  return response.data
}

export async function adminListAffiliateAgents(
  page: number,
  pageSize: number,
  keyword?: string
): Promise<AffiliateApiResponse<AffiliatePage<AffiliateAgentRecord>>> {
  const response = await api.get(
    `/api/affiliate/agents?${pageParams(page, pageSize, keyword)}`
  )
  return response.data
}

export async function adminListAffiliateInvitations(
  page: number,
  pageSize: number,
  keyword?: string
): Promise<AffiliateApiResponse<AffiliatePage<AffiliateInvitationRecord>>> {
  const response = await api.get(
    `/api/affiliate/invitations?${pageParams(page, pageSize, keyword)}`
  )
  return response.data
}

export async function adminListAffiliateRedemptions(
  page: number,
  pageSize: number,
  keyword?: string
): Promise<AffiliateApiResponse<AffiliatePage<AffiliateRedemptionRecord>>> {
  const response = await api.get(
    `/api/affiliate/redemptions?${pageParams(page, pageSize, keyword)}`
  )
  return response.data
}

export async function adminListAffiliateCommissions(
  page: number,
  pageSize: number,
  keyword?: string
): Promise<AffiliateApiResponse<AffiliatePage<AffiliateCommissionRecord>>> {
  const response = await api.get(
    `/api/affiliate/commissions?${pageParams(page, pageSize, keyword)}`
  )
  return response.data
}

export async function adminListAffiliateConversions(
  page: number,
  pageSize: number
): Promise<AffiliateApiResponse<AffiliatePage<AffiliateConversionRecord>>> {
  const response = await api.get(
    `/api/affiliate/conversions?${pageParams(page, pageSize)}`
  )
  return response.data
}

export async function adminListAffiliateWithdrawals(
  page: number,
  pageSize: number,
  status?: AffiliateWithdrawalStatus
): Promise<AffiliateApiResponse<AffiliatePage<AffiliateWithdrawalRecord>>> {
  const params = pageParams(page, pageSize)
  if (status) {
    params.set('status', status)
  }
  const response = await api.get(`/api/affiliate/withdrawals?${params}`)
  return response.data
}

export async function adminReviewAffiliateWithdrawal(
  withdrawalId: number,
  decision: 'pay' | 'reject',
  adminNote: string
): Promise<AffiliateApiResponse<AffiliateWithdrawalRecord>> {
  const response = await api.post(
    `/api/affiliate/withdrawals/${withdrawalId}/${decision}`,
    { admin_note: adminNote }
  )
  return response.data
}
