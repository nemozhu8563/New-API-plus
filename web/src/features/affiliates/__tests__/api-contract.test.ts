import assert from 'node:assert/strict'

import { afterEach, describe, test } from 'vitest'

import { api } from '@/lib/api'

import {
  adminGetAffiliateAgent,
  adminListAffiliateAgents,
  adminListAffiliateCommissions,
  adminListAffiliateConversions,
  adminListAffiliateInvitations,
  adminListAffiliateRedemptions,
  adminListAffiliateWithdrawals,
  adminReviewAffiliateWithdrawal,
  adminUpdateAffiliateAgent,
  convertAffiliateCashback,
  createAffiliateWithdrawal,
  getAffiliateCommissions,
  getAffiliateConversions,
  getAffiliateInvitees,
  getAffiliateRedemptions,
  getAffiliateSummary,
  getAffiliateWithdrawals,
} from '../api'

interface AffiliateApiCall {
  method: 'GET' | 'POST' | 'PUT'
  url: string
  body?: unknown
}

const originalGet = api.get
const originalPost = api.post
const originalPut = api.put
let calls: AffiliateApiCall[] = []

function installAffiliateApiRecorder() {
  api.get = (async (url: string) => {
    calls.push({ method: 'GET', url })
    return {
      data: {
        success: true,
        data: { page: 1, page_size: 20, total: 0, items: [] },
      },
    }
  }) as unknown as typeof api.get
  api.post = (async (url: string, body?: unknown) => {
    calls.push({ method: 'POST', url, body })
    return { data: { success: true, data: {} } }
  }) as unknown as typeof api.post
  api.put = (async (url: string, body?: unknown) => {
    calls.push({ method: 'PUT', url, body })
    return { data: { success: true, data: {} } }
  }) as unknown as typeof api.put
}

afterEach(() => {
  api.get = originalGet
  api.post = originalPost
  api.put = originalPut
  calls = []
})

describe('affiliate API contract', () => {
  test('uses authenticated self endpoints with explicit pagination', async () => {
    installAffiliateApiRecorder()

    await getAffiliateSummary()
    await getAffiliateInvitees(2, 25)
    await getAffiliateRedemptions(3, 10)
    await getAffiliateCommissions(4, 50)
    await getAffiliateConversions(5, 20)
    await getAffiliateWithdrawals(6, 100)

    assert.deepEqual(
      calls.map((call) => call.url),
      [
        '/api/user/affiliate/summary',
        '/api/user/affiliate/invitees?p=2&page_size=25',
        '/api/user/affiliate/redemptions?p=3&page_size=10',
        '/api/user/affiliate/commissions?p=4&page_size=50',
        '/api/user/affiliate/conversions?p=5&page_size=20',
        '/api/user/affiliate/withdrawals?p=6&page_size=100',
      ]
    )
  })

  test('trims admin keywords and sends withdrawal status only when selected', async () => {
    installAffiliateApiRecorder()
    const keyword = '  Alice + Bob  '

    await adminGetAffiliateAgent(42)
    await adminListAffiliateAgents(1, 20, keyword)
    await adminListAffiliateInvitations(2, 20, keyword)
    await adminListAffiliateRedemptions(3, 20, keyword)
    await adminListAffiliateCommissions(4, 20, keyword)
    await adminListAffiliateConversions(5, 20)
    await adminListAffiliateWithdrawals(6, 20)
    await adminListAffiliateWithdrawals(7, 20, 'pending')

    assert.deepEqual(
      calls.map((call) => call.url),
      [
        '/api/affiliate/agents/42',
        '/api/affiliate/agents?p=1&page_size=20&keyword=Alice+%2B+Bob',
        '/api/affiliate/invitations?p=2&page_size=20&keyword=Alice+%2B+Bob',
        '/api/affiliate/redemptions?p=3&page_size=20&keyword=Alice+%2B+Bob',
        '/api/affiliate/commissions?p=4&page_size=20&keyword=Alice+%2B+Bob',
        '/api/affiliate/conversions?p=5&page_size=20',
        '/api/affiliate/withdrawals?p=6&page_size=20',
        '/api/affiliate/withdrawals?p=7&page_size=20&status=pending',
      ]
    )
  })

  test('sends quota, agent policy, and review decisions in mutation payloads', async () => {
    installAffiliateApiRecorder()

    await convertAffiliateCashback(750_000)
    await createAffiliateWithdrawal({
      amount_quota: 500_000,
      note: 'PayPal',
    })
    await adminUpdateAffiliateAgent(42, {
      enabled: true,
      commission_rate_bps: 825,
      cash_withdrawal_enabled: true,
    })
    await adminReviewAffiliateWithdrawal(7, 'pay', 'paid offline')
    await adminReviewAffiliateWithdrawal(8, 'reject', 'missing details')

    assert.deepEqual(calls, [
      {
        method: 'POST',
        url: '/api/user/affiliate/convert',
        body: { amount_quota: 750_000 },
      },
      {
        method: 'POST',
        url: '/api/user/affiliate/withdrawals',
        body: { amount_quota: 500_000, note: 'PayPal' },
      },
      {
        method: 'PUT',
        url: '/api/affiliate/agents/42',
        body: {
          enabled: true,
          commission_rate_bps: 825,
          cash_withdrawal_enabled: true,
        },
      },
      {
        method: 'POST',
        url: '/api/affiliate/withdrawals/7/pay',
        body: { admin_note: 'paid offline' },
      },
      {
        method: 'POST',
        url: '/api/affiliate/withdrawals/8/reject',
        body: { admin_note: 'missing details' },
      },
    ])
  })
})
