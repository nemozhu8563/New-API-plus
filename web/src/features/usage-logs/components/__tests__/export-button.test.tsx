import assert from 'node:assert/strict'
import { after, afterEach, describe, test } from 'node:test'

import { Window } from 'happy-dom'

import { api } from '@/lib/api'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MouseEvent',
  'PointerEvent',
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
const { UsageLogsExportButton } = await import('../usage-logs-export-button')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Export CSV': 'Export CSV',
        'Failed to export usage logs': 'Failed to export usage logs',
        'Usage logs exported': 'Usage logs exported',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const originalGet = api.get
const originalCreateObjectURL = URL.createObjectURL
const originalRevokeObjectURL = URL.revokeObjectURL
let activeRender: {
  container: HTMLDivElement
  root: ReturnType<typeof createRoot>
} | null = null

async function renderExportButton() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <UsageLogsExportButton
          logCategory='common'
          isAdmin
          searchParams={{ model: 'gpt-5' }}
          columnFilters={[]}
        />
      </I18nextProvider>
    )
  })
  activeRender = { container, root }
}

afterEach(async () => {
  if (activeRender) {
    await act(async () => activeRender?.root.unmount())
    activeRender.container.remove()
    activeRender = null
  }
  api.get = originalGet
  URL.createObjectURL = originalCreateObjectURL
  URL.revokeObjectURL = originalRevokeObjectURL
  document.body.replaceChildren()
})

describe('usage log export button', () => {
  after(() => {
    domWindow.close()
  })

  test('disables while exporting and prevents a duplicate request', async () => {
    let resolveRequest:
      | ((value: { data: Blob; headers: Record<string, string> }) => void)
      | undefined
    let requestCount = 0
    api.get = (() => {
      requestCount += 1
      return new Promise<{
        data: Blob
        headers: Record<string, string>
      }>((resolve) => {
        resolveRequest = resolve
      })
    }) as unknown as typeof api.get
    URL.createObjectURL = () => 'blob:usage-logs'
    URL.revokeObjectURL = () => undefined

    await renderExportButton()
    const button = document.querySelector<HTMLButtonElement>(
      'button[aria-label="Export CSV"]'
    )
    assert.ok(button)

    await act(async () => {
      button.click()
      await Promise.resolve()
    })
    assert.equal(button.disabled, true)
    button.click()
    assert.equal(requestCount, 1)

    await act(async () => {
      resolveRequest?.({
        data: new Blob(['id\\n1\\n']),
        headers: {
          'content-disposition': 'attachment; filename="usage-logs-common.csv"',
        },
      })
      await Promise.resolve()
    })
    assert.equal(button.disabled, false)
  })
})
