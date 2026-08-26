import assert from 'node:assert/strict'
import { after, afterEach, describe, test } from 'node:test'

import { Window } from 'happy-dom'

import { api } from '@/lib/api'

import type { Redemption } from '../../types'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLInputElement',
  'HTMLFormElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'KeyboardEvent',
  'PointerEvent',
  'MouseEvent',
  'FocusEvent',
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
const { RedemptionsProvider } = await import('../redemptions-provider')
const { RedemptionsMutateDrawer } = await import('../redemptions-mutate-drawer')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

type Deferred<T> = {
  promise: Promise<T>
  resolve: (value: T) => void
}

function deferred<T>(): Deferred<T> {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}

function redemption(id: number, name: string): Redemption {
  return {
    id,
    user_id: 1,
    name,
    key: String(id).padStart(32, '0'),
    status: 1,
    quota: 100,
    benefit_type: 'quota',
    subscription_plan_id: 0,
    subscription_plan_title: '',
    used_subscription_id: 0,
    created_time: 1,
    redeemed_time: 0,
    expired_time: 0,
    used_user_id: 0,
  }
}

const originalGet = api.get
const originalPut = api.put
let root: ReturnType<typeof createRoot> | null = null
let host: HTMLDivElement | null = null

afterEach(async () => {
  api.get = originalGet
  api.put = originalPut
  if (root) {
    await act(async () => root?.unmount())
    root = null
  }
  host?.remove()
  host = null
  document.body.replaceChildren()
})

after(() => {
  domWindow.close()
})

describe('redemptions mutate drawer', () => {
  test('blocks update submission until the current code finishes loading', async () => {
    const response = deferred<{
      data: { success: true; data: Redemption }
    }>()
    let updateCalls = 0
    api.get = (() => response.promise) as unknown as typeof api.get
    api.put = (() => {
      updateCalls += 1
      return Promise.resolve({ data: { success: true } })
    }) as unknown as typeof api.put

    const currentRow = redemption(1, 'current row')
    host = document.createElement('div')
    document.body.append(host)
    root = createRoot(host)

    await act(async () => {
      root?.render(
        <I18nextProvider i18n={i18n}>
          <RedemptionsProvider>
            <RedemptionsMutateDrawer
              open
              onOpenChange={() => undefined}
              currentRow={currentRow}
            />
          </RedemptionsProvider>
        </I18nextProvider>
      )
    })

    const saveButton = document.querySelector<HTMLButtonElement>(
      'button[form="redemption-form"]'
    )
    const form = document.querySelector<HTMLFormElement>('#redemption-form')
    assert.ok(saveButton)
    assert.ok(form)
    assert.equal(saveButton.disabled, true)

    await act(async () => {
      form.dispatchEvent(
        new Event('submit', { bubbles: true, cancelable: true })
      )
      await Promise.resolve()
    })
    assert.equal(updateCalls, 0)

    response.resolve({ data: { success: true, data: currentRow } })
    await act(async () => {
      await response.promise
      await Promise.resolve()
    })

    assert.equal(saveButton.disabled, false)
  })

  test('ignores an earlier edit response after switching to another code', async () => {
    const first = deferred<{ data: { success: true; data: Redemption } }>()
    const second = deferred<{ data: { success: true; data: Redemption } }>()
    api.get = ((url: string) => {
      if (url === '/api/redemption/1') return first.promise
      if (url === '/api/redemption/2') return second.promise
      throw new Error(`Unexpected GET ${url}`)
    }) as unknown as typeof api.get

    const firstRow = redemption(1, 'first row')
    const secondRow = redemption(2, 'second row')
    host = document.createElement('div')
    document.body.append(host)
    root = createRoot(host)

    const render = async (currentRow: Redemption) => {
      await act(async () => {
        root?.render(
          <I18nextProvider i18n={i18n}>
            <RedemptionsProvider>
              <RedemptionsMutateDrawer
                open
                onOpenChange={() => undefined}
                currentRow={currentRow}
              />
            </RedemptionsProvider>
          </I18nextProvider>
        )
      })
    }

    await render(firstRow)
    await render(secondRow)

    second.resolve({ data: { success: true, data: secondRow } })
    await act(async () => {
      await second.promise
      await Promise.resolve()
    })

    const nameInput = document.querySelector<HTMLInputElement>(
      'input[placeholder="Enter a name"]'
    )
    assert.ok(nameInput)
    assert.equal(nameInput.value, 'second row')

    first.resolve({ data: { success: true, data: firstRow } })
    await act(async () => {
      await first.promise
      await Promise.resolve()
    })

    assert.equal(nameInput.value, 'second row')
  })
})
