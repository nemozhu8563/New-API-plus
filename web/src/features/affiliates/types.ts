export type AffiliateRewardType = 'ordinary_first' | 'selected_agent'
export type AffiliateRewardDestination = 'site_balance' | 'cashback'
export type AffiliateWithdrawalStatus = 'pending' | 'paid' | 'rejected'

export interface AffiliateApiResponse<T = unknown> {
  success?: boolean
  message?: string
  data?: T
}

export interface AffiliatePage<T> {
  page: number
  page_size: number
  total: number
  items: T[]
}

export interface AffiliateSummary {
  is_agent: boolean
  enabled: boolean
  commission_rate_bps: number
  cash_withdrawal_enabled: boolean
  available_quota: number
  pending_withdrawal_quota: number
  converted_quota: number
  withdrawn_quota: number
  total_commission_quota: number
  invitee_count: number
  ordinary_reward_quota: number
  total_reward_quota: number
  redemption_count: number
  redeemed_quota: number
}

export interface AffiliateAgent {
  user_id: number
  enabled: boolean
  commission_rate_bps: number
  cash_withdrawal_enabled: boolean
  available_quota: number
  pending_withdrawal_quota: number
  converted_quota: number
  withdrawn_quota: number
  total_commission_quota: number
  created_at: number
  updated_at: number
}

export interface AffiliateAgentRecord extends AffiliateAgent {
  username: string
  display_name: string
}

export interface AffiliateCommissionRecord {
  id: number
  inviter_user_id: number
  invitee_user_id: number
  redemption_id: number
  source_quota: number
  rate_bps: number
  commission_quota: number
  reward_type: AffiliateRewardType
  destination: AffiliateRewardDestination
  created_at: number
  inviter_username: string
  invitee_username: string
  redemption_name: string
}

export interface AffiliateInviteeRecord {
  user_id: number
  username: string
  display_name: string
  created_at: number
  redemption_count: number
  redeemed_quota: number
  reward_quota: number
}

export interface AffiliateInvitationRecord {
  inviter_user_id: number
  inviter_username: string
  invitee_user_id: number
  invitee_username: string
  invitee_name: string
  created_at: number
  redemption_count: number
  redeemed_quota: number
  reward_quota: number
}

export interface AffiliateRedemptionRecord {
  redemption_id: number
  inviter_user_id: number
  inviter_username: string
  invitee_user_id: number
  invitee_username: string
  source_quota: number
  redeemed_at: number
  reward_type: AffiliateRewardType | ''
  destination: AffiliateRewardDestination | ''
  rate_bps: number
  reward_quota: number
}

export interface AffiliateConversionRecord {
  id: number
  agent_user_id: number
  amount_quota: number
  created_at: number
  agent_username: string
}

export interface AffiliateWithdrawalRecord {
  id: number
  agent_user_id: number
  amount_quota: number
  status: AffiliateWithdrawalStatus
  applicant_note: string
  admin_note: string
  reviewer_user_id: number
  created_at: number
  reviewed_at: number
  paid_at: number
  agent_username: string
}

export interface AffiliateAgentConfigRequest {
  enabled: boolean
  commission_rate_bps: number
  cash_withdrawal_enabled: boolean
}

export interface AffiliateWithdrawalRequest {
  amount_quota: number
  note: string
}
