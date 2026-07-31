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
import assert from 'node:assert/strict'
import { after, afterEach, describe, test } from 'node:test'

import { Window } from 'happy-dom'

import type {
  AffiliateAgentRecord,
  AffiliateWithdrawalRecord,
} from '@/features/affiliates'
import { api } from '@/lib/api'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLInputElement',
  'HTMLTextAreaElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MouseEvent',
  'KeyboardEvent',
  'FocusEvent',
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
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { AffiliateAdmin } = await import('../admin')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Affiliate management': 'Affiliate management',
        'Admin note': 'Admin note',
        'Configure agent': 'Configure agent',
        'Configure selected agent': 'Configure selected agent',
        'Confirm paid': 'Confirm paid',
        'No affiliate records found': 'No affiliate records found',
        Save: 'Save',
        Withdrawals: 'Withdrawals',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

interface ApiCall {
  method: 'GET' | 'POST' | 'PUT'
  url: string
  body?: unknown
}

interface AdminApiFixture {
  agents?: AffiliateAgentRecord[]
  withdrawals?: AffiliateWithdrawalRecord[]
}

const originalGet = api.get
const originalPost = api.post
const originalPut = api.put
let apiCalls: ApiCall[] = []
let activeRender: Awaited<ReturnType<typeof renderAffiliateAdmin>> | null = null

function installAdminApi(fixture: AdminApiFixture = {}) {
  api.get = (async (url: string) => {
    apiCalls.push({ method: 'GET', url })
    const items = url.startsWith('/api/affiliate/withdrawals?')
      ? (fixture.withdrawals ?? [])
      : (fixture.agents ?? [])
    const params = new URL(url, 'https://example.com').searchParams
    return {
      data: {
        success: true,
        data: {
          page: Number(params.get('p') ?? 1),
          page_size: Number(params.get('page_size') ?? 20),
          total: items.length,
          items,
        },
      },
    }
  }) as unknown as typeof api.get
  api.put = (async (url: string, body: unknown) => {
    apiCalls.push({ method: 'PUT', url, body })
    return { data: { success: true, data: {} } }
  }) as unknown as typeof api.put
  api.post = (async (url: string, body: unknown) => {
    apiCalls.push({ method: 'POST', url, body })
    return { data: { success: true, data: {} } }
  }) as unknown as typeof api.post
}

function changeInputValue(input: HTMLInputElement, value: string) {
  const valueSetter = Object.getOwnPropertyDescriptor(
    domWindow.HTMLInputElement.prototype,
    'value'
  )?.set
  assert.ok(valueSetter)
  valueSetter.call(input, value)
  input.dispatchEvent(
    new domWindow.Event('input', { bubbles: true }) as unknown as Event
  )
}

function changeTextareaValue(textarea: HTMLTextAreaElement, value: string) {
  const valueSetter = Object.getOwnPropertyDescriptor(
    domWindow.HTMLTextAreaElement.prototype,
    'value'
  )?.set
  assert.ok(valueSetter)
  valueSetter.call(textarea, value)
  textarea.dispatchEvent(
    new domWindow.Event('input', { bubbles: true }) as unknown as Event
  )
}

function findButton(label: string, container: ParentNode = document) {
  return [...container.querySelectorAll<HTMLButtonElement>('button')].find(
    (button) => button.textContent === label
  )
}

async function waitForBodyCondition(
  description: string,
  condition: () => boolean
) {
  await act(async () => {
    if (condition()) {
      return
    }

    await new Promise<void>((resolve, reject) => {
      let settled = false
      const finish = () => {
        if (settled) {
          return
        }
        settled = true
        domWindow.clearTimeout(timeout)
        observer.disconnect()
        resolve()
      }
      const observer = new MutationObserver(() => {
        if (condition()) {
          finish()
        }
      })
      const timeout = domWindow.setTimeout(() => {
        settled = true
        observer.disconnect()
        reject(
          new Error(
            `Timed out waiting for ${description}; body=${document.body.textContent}`
          )
        )
      }, 1000)
      observer.observe(document.body, {
        childList: true,
        characterData: true,
        subtree: true,
      })
      if (condition()) {
        finish()
      }
    })
  })
}

async function waitForQueries(queryClient: InstanceType<typeof QueryClient>) {
  await act(async () => {
    if (queryClient.isFetching() !== 0) {
      await new Promise<void>((resolve, reject) => {
        let settled = false
        let unsubscribe: () => void = () => undefined
        const timeout = domWindow.setTimeout(() => {
          settled = true
          unsubscribe()
          reject(new Error('Timed out waiting for affiliate queries'))
        }, 1000)
        const finish = () => {
          if (settled || queryClient.isFetching() !== 0) {
            return
          }
          settled = true
          domWindow.clearTimeout(timeout)
          unsubscribe()
          resolve()
        }
        unsubscribe = queryClient.getQueryCache().subscribe(finish)
        finish()
      })
    }
    await new Promise<void>((resolve) => setImmediate(resolve))
  })
}

async function renderAffiliateAdmin() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  })

  await act(async () => {
    root.render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <AffiliateAdmin />
        </I18nextProvider>
      </QueryClientProvider>
    )
  })

  return { container, queryClient, root }
}

afterEach(async () => {
  if (activeRender) {
    await act(async () => activeRender?.root.unmount())
    activeRender.container.remove()
    activeRender.queryClient.clear()
    activeRender = null
  }
  api.get = originalGet
  api.post = originalPost
  api.put = originalPut
  apiCalls = []
})

describe('affiliate admin interactions', () => {
  after(() => {
    domWindow.close()
  })

  test('rejects an out-of-range selected-agent rate without calling the API', async () => {
    installAdminApi()
    activeRender = await renderAffiliateAdmin()

    const configureButton = findButton('Configure agent')
    assert.ok(configureButton)
    await act(async () => configureButton.click())

    const userIdInput = document.querySelector<HTMLInputElement>(
      '#affiliate-agent-user-id'
    )
    const rateInput = document.querySelector<HTMLInputElement>(
      '#affiliate-agent-rate'
    )
    assert.ok(userIdInput)
    assert.ok(rateInput)
    await act(async () => {
      changeInputValue(userIdInput, '42')
      changeInputValue(rateInput, '4.99')
    })

    const saveButton = findButton('Save')
    assert.ok(saveButton)
    await act(async () => saveButton.click())

    assert.equal(
      apiCalls.some((call) => call.method === 'PUT'),
      false
    )
    assert.equal(
      document.body.textContent?.includes('Configure selected agent'),
      true
    )
  })

  test('associates selected-agent switches with their visible labels', async () => {
    installAdminApi()
    activeRender = await renderAffiliateAdmin()

    const configureButton = findButton('Configure agent')
    assert.ok(configureButton)
    await act(async () => configureButton.click())

    const enabledSwitch = document.querySelector('#affiliate-agent-enabled')
    const withdrawalSwitch = document.querySelector(
      '#affiliate-agent-withdrawal-enabled'
    )
    const enabledLabel = document.querySelector(
      'label[for="affiliate-agent-enabled"]'
    )
    const withdrawalLabel = document.querySelector(
      'label[for="affiliate-agent-withdrawal-enabled"]'
    )

    assert.ok(enabledSwitch)
    assert.ok(withdrawalSwitch)
    assert.equal(enabledLabel?.textContent, 'Enable selected agent')
    assert.equal(withdrawalLabel?.textContent, 'Allow withdrawal requests')
  })

  test('saves a valid rate as basis points and preserves the withdrawal setting', async () => {
    installAdminApi()
    activeRender = await renderAffiliateAdmin()

    const configureButton = findButton('Configure agent')
    assert.ok(configureButton)
    await act(async () => configureButton.click())

    const userIdInput = document.querySelector<HTMLInputElement>(
      '#affiliate-agent-user-id'
    )
    const rateInput = document.querySelector<HTMLInputElement>(
      '#affiliate-agent-rate'
    )
    const switches = [
      ...document.querySelectorAll<HTMLButtonElement>('[data-slot="switch"]'),
    ]
    assert.ok(userIdInput)
    assert.ok(rateInput)
    assert.equal(switches.length, 2)
    await act(async () => {
      changeInputValue(userIdInput, '42')
      changeInputValue(rateInput, '7.25')
      switches[1].click()
    })

    const saveButton = findButton('Save')
    assert.ok(saveButton)
    await act(async () => saveButton.click())
    await waitForBodyCondition(
      'the agent dialog to close',
      () =>
        document.body.textContent?.includes('Configure selected agent') ===
        false
    )

    const putCalls = apiCalls.filter((call) => call.method === 'PUT')
    assert.deepEqual(putCalls, [
      {
        method: 'PUT',
        url: '/api/affiliate/agents/42',
        body: {
          enabled: true,
          commission_rate_bps: 725,
          cash_withdrawal_enabled: true,
        },
      },
    ])
  })

  test('requires withdrawal review confirmation and enforces the 500-character note limit', async () => {
    installAdminApi({
      withdrawals: [
        {
          id: 7,
          agent_user_id: 42,
          amount_quota: 1_000_000,
          status: 'pending',
          applicant_note: 'PayPal: agent@example.com',
          admin_note: '',
          reviewer_user_id: 0,
          created_at: 1_750_000_000,
          reviewed_at: 0,
          paid_at: 0,
          agent_username: 'selected-agent',
        },
      ],
    })
    activeRender = await renderAffiliateAdmin()

    const withdrawalsTab = findButton('Withdrawals')
    assert.ok(withdrawalsTab)
    await act(async () => withdrawalsTab.click())
    await waitForQueries(activeRender.queryClient)
    assert.equal(document.body.textContent?.includes('selected-agent'), true)

    const tableConfirmButton = findButton('Confirm paid')
    assert.ok(tableConfirmButton)
    assert.equal(
      apiCalls.some((call) => call.method === 'POST'),
      false
    )
    await act(async () => tableConfirmButton.click())

    const reviewDialog = document.querySelector<HTMLElement>('[role="dialog"]')
    const noteInput = document.querySelector<HTMLTextAreaElement>(
      '#affiliate-review-note'
    )
    assert.ok(reviewDialog)
    assert.ok(noteInput)
    const dialogConfirmButton = findButton('Confirm paid', reviewDialog)
    assert.ok(dialogConfirmButton)

    await act(async () => changeTextareaValue(noteInput, '界'.repeat(501)))
    assert.equal(dialogConfirmButton.disabled, true)
    assert.equal(
      apiCalls.some((call) => call.method === 'POST'),
      false
    )

    await act(async () => changeTextareaValue(noteInput, '界'.repeat(500)))
    assert.equal(dialogConfirmButton.disabled, false)
    await act(async () => dialogConfirmButton.click())
    await waitForBodyCondition(
      'the withdrawal review dialog to close',
      () => document.querySelector('#affiliate-review-note') === null
    )

    const postCalls = apiCalls.filter((call) => call.method === 'POST')
    assert.deepEqual(postCalls, [
      {
        method: 'POST',
        url: '/api/affiliate/withdrawals/7/pay',
        body: { admin_note: '界'.repeat(500) },
      },
    ])
  })
})
