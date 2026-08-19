import assert from 'node:assert/strict'
import { after, afterEach, describe, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window({ url: 'https://test.tryvalo.com/wallet' })
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
const { api } = await import('@/lib/api')
const { StripeBillingPortalButton } =
  await import('../stripe-billing-portal-button')

const originalAdapter = api.defaults.adapter
const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Manage billing': 'Manage billing',
        'Opening billing portal...': 'Opening billing portal...',
        'Unable to open billing portal': 'Unable to open billing portal',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

async function renderButton() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <StripeBillingPortalButton />
      </I18nextProvider>
    )
  })
  const button = container.querySelector('button')
  assert.ok(button)
  return { button, container, root }
}

afterEach(() => {
  api.defaults.adapter = originalAdapter
  domWindow.location.href = 'https://test.tryvalo.com/wallet'
})

after(() => {
  domWindow.close()
})

describe('Stripe billing portal button', () => {
  test('creates a portal session and redirects in the current tab', async () => {
    let requestUrl = ''
    let requestMethod = ''
    api.defaults.adapter = async (config) => {
      requestUrl = config.url || ''
      requestMethod = config.method || ''
      return {
        data: {
          success: true,
          data: {
            portal_url: 'https://billing.stripe.com/p/session/test',
          },
        },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }

    const { button, container, root } = await renderButton()
    await act(async () => {
      button.click()
      await Promise.resolve()
      await Promise.resolve()
    })

    assert.equal(requestUrl, '/api/subscription/stripe/portal')
    assert.equal(requestMethod, 'post')
    assert.equal(
      domWindow.location.href,
      'https://billing.stripe.com/p/session/test'
    )

    await act(async () => root.unmount())
    container.remove()
  })

  test('restores the button after a failed portal response', async () => {
    api.defaults.adapter = async (config) => ({
      data: { success: false, message: 'portal unavailable' },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    })

    const { button, container, root } = await renderButton()
    await act(async () => {
      button.click()
      await Promise.resolve()
      await Promise.resolve()
    })

    assert.equal(button.disabled, false)
    assert.equal(button.textContent?.includes('Manage billing'), true)
    assert.equal(domWindow.location.href, 'https://test.tryvalo.com/wallet')

    await act(async () => root.unmount())
    container.remove()
  })
})
