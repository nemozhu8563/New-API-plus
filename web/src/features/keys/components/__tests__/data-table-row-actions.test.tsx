import assert from 'node:assert/strict'

import { Window } from 'happy-dom'
import { afterAll as after, afterEach, describe, test } from 'vitest'

import { api } from '@/lib/api'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLImageElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MouseEvent',
  'PointerEvent',
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
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { TooltipProvider } = await import('@/components/ui/tooltip')
const { ApiKeysProvider, useApiKeys } = await import('../api-keys-provider')
const { DataTableRowActions } = await import('../data-table-row-actions')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Copy Connection Info': 'Copy Connection Info',
        'Copy Key': 'Copy Key',
        'Import to CC Switch': 'Import to CC Switch',
        Delete: 'Delete',
        Disable: 'Disable',
        Edit: 'Edit',
        'Open menu': 'Open menu',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const apiKey = {
  id: 7,
  name: 'Test key',
  key: 'sk-****',
  status: 1,
  remain_quota: 100,
  used_quota: 0,
  unlimited_quota: false,
  expired_time: -1,
  created_time: 0,
  accessed_time: 0,
  group: 'default',
  cross_group_retry: false,
  model_limits_enabled: false,
  model_limits: '',
  allow_ips: '',
}

const originalPost = api.post
let apiCalls: string[] = []
let activeRender: Awaited<ReturnType<typeof renderRowActions>> | null = null

function ApiKeyDialogState() {
  const { open, resolvedKey } = useApiKeys()
  return (
    <output
      data-testid='api-key-dialog-state'
      data-open={open ?? ''}
      data-key={resolvedKey}
    />
  )
}

async function renderRowActions() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <TooltipProvider>
          <ApiKeysProvider>
            <DataTableRowActions
              row={
                {
                  original: apiKey,
                } as Parameters<typeof DataTableRowActions>[0]['row']
              }
            />
            <ApiKeyDialogState />
          </ApiKeysProvider>
        </TooltipProvider>
      </I18nextProvider>
    )
  })

  return { container, root }
}

afterEach(async () => {
  if (activeRender) {
    await act(async () => activeRender?.root.unmount())
    activeRender.container.remove()
    activeRender = null
  }
  api.post = originalPost
  apiCalls = []
  document.body.replaceChildren()
})

describe('API key row actions', () => {
  after(() => {
    domWindow.close()
  })

  test('opening the menu does not resolve the real key or expose chat imports', async () => {
    api.post = (async (url: string) => {
      apiCalls.push(url)
      return { data: { success: true, data: { key: 'secret' } } }
    }) as typeof api.post
    activeRender = await renderRowActions()

    const menuButton = document.querySelector<HTMLButtonElement>(
      'button[aria-label="Open menu"]'
    )
    assert.ok(menuButton)
    await act(async () => menuButton.click())

    assert.equal(document.body.textContent?.includes('Copy Key'), true)
    assert.equal(document.body.textContent?.includes('Chat'), false)
    assert.deepEqual(apiCalls, [])
  })

  test('the CC Switch action resolves the key only after an explicit click', async () => {
    api.post = (async (url: string) => {
      apiCalls.push(url)
      return { data: { success: true, data: { key: 'secret' } } }
    }) as typeof api.post
    activeRender = await renderRowActions()

    assert.deepEqual(apiCalls, [])
    const ccSwitchButton = document.querySelector<HTMLButtonElement>(
      'button[aria-label="Import to CC Switch"]'
    )
    assert.ok(ccSwitchButton)
    await act(async () => {
      ccSwitchButton.click()
      await new Promise<void>((resolve) => setImmediate(resolve))
    })

    assert.deepEqual(apiCalls, ['/api/token/7/key'])
    const state = document.querySelector<HTMLOutputElement>(
      '[data-testid="api-key-dialog-state"]'
    )
    assert.equal(state?.dataset.open, 'cc-switch')
    assert.equal(state?.dataset.key, 'sk-secret')
  })
})
