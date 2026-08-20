import assert from 'node:assert/strict'
import { after, afterEach, describe, test } from 'node:test'

import { Window } from 'happy-dom'

import type { LegalDocumentResponse } from '../types'

const domWindow = new Window({ url: 'https://test.tryvalo.com/user-agreement' })
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLAnchorElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'ResizeObserver',
  'Image',
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

const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { createMemoryHistory, createRootRoute, createRouter, RouterProvider } =
  await import('@tanstack/react-router')
const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { api } = await import('@/lib/api')
const { LegalDocument } = await import('../legal-document')

const originalAdapter = api.defaults.adapter
const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

async function flushQueries() {
  await act(async () => {
    await Promise.resolve()
    await Promise.resolve()
  })
}

afterEach(() => {
  api.defaults.adapter = originalAdapter
  document.body.replaceChildren()
  window.localStorage.clear()
})

after(() => {
  domWindow.close()
})

describe('localized legal documents', () => {
  test('refetches with a separate query key after the interface language changes', async () => {
    api.defaults.adapter = async (config) => ({
      data:
        config.url === '/api/status'
          ? { success: true, data: {} }
          : { success: true, data: '' },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    })
    const requestedLocales: string[] = []
    const i18n = createInstance()
    await i18n.use(initReactI18next).init({
      lng: 'en',
      fallbackLng: 'en',
      resources: {
        en: { translation: {} },
        fr: { translation: {} },
      },
    })
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const rootRoute = createRootRoute({
      component: () => (
        <LegalDocument
          title='Terms of Service'
          queryKey='user-agreement'
          fetchDocument={async (locale: string) => {
            requestedLocales.push(locale)
            return { success: true, data: `${locale} terms` }
          }}
          emptyMessage='Missing terms'
        />
      ),
    })
    const router = createRouter({
      routeTree: rootRoute,
      history: createMemoryHistory({ initialEntries: ['/'] }),
    })
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <QueryClientProvider client={queryClient}>
            <RouterProvider router={router} />
          </QueryClientProvider>
        </I18nextProvider>
      )
    })
    await flushQueries()
    assert.deepEqual(requestedLocales, ['en'])

    await act(async () => {
      await i18n.changeLanguage('fr')
    })
    await flushQueries()

    assert.deepEqual(requestedLocales, ['en', 'fr'])
    assert.equal(
      queryClient.getQueryData<LegalDocumentResponse>(['user-agreement', 'en'])
        ?.data,
      'en terms'
    )
    assert.equal(
      queryClient.getQueryData<LegalDocumentResponse>(['user-agreement', 'fr'])
        ?.data,
      'fr terms'
    )

    await act(async () => root.unmount())
    queryClient.clear()
    container.remove()
  })
})
