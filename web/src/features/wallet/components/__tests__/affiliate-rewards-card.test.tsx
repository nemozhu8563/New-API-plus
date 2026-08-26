import assert from 'node:assert/strict'

import { Window } from 'happy-dom'
import { afterAll as after, describe, test } from 'vitest'

import type { AffiliateSummary } from '@/features/affiliates'
import type { UserWalletData } from '@/features/wallet/types'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLInputElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { AffiliateRewardsCard } = await import('../affiliate-rewards-card')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        '5% site balance on each invitee’s first redemption':
          '5% site balance on each invitee’s first redemption',
        '8.00% permanent cashback': '8.00% permanent cashback',
        'Available cashback': 'Available cashback',
        'Convert to balance': 'Convert to balance',
        Converted: 'Converted',
        'Copy referral link': 'Copy referral link',
        'First redemption rewards': 'First redemption rewards',
        'Invited accounts': 'Invited accounts',
        'Pending withdrawal': 'Pending withdrawal',
        Redemptions: 'Redemptions',
        'Redeemed face value': 'Redeemed face value',
        'Referral Program': 'Referral Program',
        'Referral reward transfer is disabled until the administrator confirms compliance terms.':
          'Referral reward transfer is disabled until the administrator confirms compliance terms.',
        'Request withdrawal': 'Request withdrawal',
        'Selected agent': 'Selected agent',
        'Standard inviter': 'Standard inviter',
        'Transfer legacy rewards': 'Transfer legacy rewards',
        'View records': 'View records',
        Withdrawn: 'Withdrawn',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const disabledAgentSummary: AffiliateSummary = {
  is_agent: true,
  enabled: false,
  commission_rate_bps: 800,
  cash_withdrawal_enabled: true,
  available_quota: 5000,
  pending_withdrawal_quota: 0,
  converted_quota: 0,
  withdrawn_quota: 0,
  total_commission_quota: 5000,
  invitee_count: 2,
  ordinary_reward_quota: 250,
  total_reward_quota: 5250,
  redemption_count: 3,
  redeemed_quota: 15000,
}

const ordinarySummary: AffiliateSummary = {
  is_agent: false,
  enabled: false,
  commission_rate_bps: 0,
  cash_withdrawal_enabled: false,
  available_quota: 0,
  pending_withdrawal_quota: 0,
  converted_quota: 0,
  withdrawn_quota: 0,
  total_commission_quota: 0,
  invitee_count: 3,
  ordinary_reward_quota: 250,
  total_reward_quota: 250,
  redemption_count: 4,
  redeemed_quota: 5000,
}

const activeAgentSummary: AffiliateSummary = {
  ...disabledAgentSummary,
  enabled: true,
  commission_rate_bps: 800,
}

const walletUser: UserWalletData = {
  id: 1,
  username: 'affiliate-user',
  quota: 0,
  used_quota: 0,
  request_count: 0,
  aff_quota: 1000,
  aff_history_quota: 1000,
  aff_count: 1,
  group: 'default',
}

async function renderRewardsCard(
  overrides: Partial<Parameters<typeof AffiliateRewardsCard>[0]> = {}
) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  const props: Parameters<typeof AffiliateRewardsCard>[0] = {
    user: null,
    summary: ordinarySummary,
    affiliateLink: 'https://example.com/register?aff=test',
    onViewDetails: () => undefined,
    onConvertCashback: () => undefined,
    onRequestWithdrawal: () => undefined,
    onTransferLegacyRewards: () => undefined,
    ...overrides,
  }

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <AffiliateRewardsCard {...props} />
      </I18nextProvider>
    )
  })

  return { container, root }
}

function findButton(container: HTMLElement, label: string) {
  return [...container.querySelectorAll('button')].find(
    (button) => button.textContent === label
  )
}

describe('affiliate rewards card', () => {
  after(() => {
    domWindow.close()
  })

  test('shows the standard policy for a disabled agent while keeping earned cashback usable', async () => {
    const { container, root } = await renderRewardsCard({
      summary: disabledAgentSummary,
    })

    assert.equal(container.textContent?.includes('Standard inviter'), true)
    assert.equal(container.textContent?.includes('Selected agent'), false)
    assert.equal(
      container.textContent?.includes(
        '5% site balance on each invitee’s first redemption'
      ),
      true
    )

    const buttonLabels = new Set(
      [...container.querySelectorAll('button')].map(
        (button) => button.textContent
      )
    )
    assert.equal(buttonLabels.has('Convert to balance'), true)
    assert.equal(buttonLabels.has('Request withdrawal'), true)

    await act(async () => root.unmount())
    container.remove()
  })

  test('shows ordinary invitation totals without agent-only money actions', async () => {
    let detailsOpenCount = 0
    const { container, root } = await renderRewardsCard({
      summary: ordinarySummary,
      onViewDetails: () => {
        detailsOpenCount += 1
      },
    })

    assert.equal(container.textContent?.includes('Standard inviter'), true)
    assert.equal(container.textContent?.includes('Invited accounts'), true)
    assert.equal(container.textContent?.includes('Redemptions'), true)
    assert.equal(container.textContent?.includes('Redeemed face value'), true)
    assert.equal(
      container.textContent?.includes('First redemption rewards'),
      true
    )
    assert.equal(findButton(container, 'Convert to balance'), undefined)
    assert.equal(findButton(container, 'Request withdrawal'), undefined)

    const viewButton = findButton(container, 'View records')
    assert.ok(viewButton)
    await act(async () => viewButton.click())
    assert.equal(detailsOpenCount, 1)

    await act(async () => root.unmount())
    container.remove()
  })

  test('shows selected-agent cashback metrics and invokes money actions', async () => {
    let convertCount = 0
    let withdrawalCount = 0
    const { container, root } = await renderRewardsCard({
      summary: activeAgentSummary,
      onConvertCashback: () => {
        convertCount += 1
      },
      onRequestWithdrawal: () => {
        withdrawalCount += 1
      },
    })

    assert.equal(container.textContent?.includes('Selected agent'), true)
    assert.equal(
      container.textContent?.includes('8.00% permanent cashback'),
      true
    )
    assert.equal(container.textContent?.includes('Available cashback'), true)
    assert.equal(container.textContent?.includes('Pending withdrawal'), true)
    assert.equal(container.textContent?.includes('Converted'), true)
    assert.equal(container.textContent?.includes('Withdrawn'), true)

    const convertButton = findButton(container, 'Convert to balance')
    const withdrawalButton = findButton(container, 'Request withdrawal')
    assert.ok(convertButton)
    assert.ok(withdrawalButton)
    await act(async () => {
      convertButton.click()
      withdrawalButton.click()
    })
    assert.equal(convertCount, 1)
    assert.equal(withdrawalCount, 1)

    await act(async () => root.unmount())
    container.remove()
  })

  test('disables every balance-moving action until compliance is confirmed', async () => {
    const { container, root } = await renderRewardsCard({
      user: walletUser,
      summary: activeAgentSummary,
      complianceConfirmed: false,
    })

    const convertButton = findButton(container, 'Convert to balance')
    const withdrawalButton = findButton(container, 'Request withdrawal')
    const legacyButton = findButton(container, 'Transfer legacy rewards')
    const viewButton = findButton(container, 'View records')
    assert.ok(convertButton)
    assert.ok(withdrawalButton)
    assert.ok(legacyButton)
    assert.ok(viewButton)
    assert.equal(convertButton.disabled, true)
    assert.equal(withdrawalButton.disabled, true)
    assert.equal(legacyButton.disabled, true)
    assert.equal(viewButton.disabled, false)
    assert.equal(
      container.textContent?.includes(
        'Referral reward transfer is disabled until the administrator confirms compliance terms.'
      ),
      true
    )

    await act(async () => root.unmount())
    container.remove()
  })
})
