import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

function matchingRuntime() {
  return {
    canonicalOrigin: window.location.origin,
    telemetryOrigin: window.location.origin,
    googleAnalyticsId: 'G-TEST123',
    clarityProjectId: 'abc123',
  }
}

async function loadTelemetry() {
  vi.resetModules()
  return import('../site-telemetry')
}

function telemetryEvents(): unknown[][] {
  return (window.dataLayer ?? []).filter(
    (entry): entry is unknown[] => Array.isArray(entry) && entry[0] === 'event'
  )
}

describe('site telemetry', () => {
  beforeEach(() => {
    window.history.replaceState({}, '', '/')
    window.localStorage.clear()
    delete window.__SITE_RUNTIME__
    delete window.__SITE_TELEMETRY_GA_CONSENT_DEFAULTED__
    delete window.dataLayer
    delete window.gtag
    delete window.clarity
    document
      .querySelectorAll(
        '#site-telemetry-ga4, #site-telemetry-clarity, link[rel="canonical"]'
      )
      .forEach((element) => element.remove())
  })

  afterEach(() => {
    window.localStorage.clear()
    window.history.replaceState({}, '', '/')
    vi.restoreAllMocks()
  })

  test('sanitizes OAuth URLs globally before GA4 initialization and on login events', async () => {
    window.history.replaceState(
      {},
      '',
      '/oauth/github?code=oauth-secret&state=state-secret#fragment-secret'
    )
    vi.spyOn(document, 'referrer', 'get').mockReturnValue(
      'https://identity.example/authorize?code=referrer-secret#referrer-fragment'
    )
    window.__SITE_RUNTIME__ = matchingRuntime()
    window.localStorage.setItem('tryvalo.analytics-consent.v1', 'granted')
    const telemetry = await loadTelemetry()

    telemetry.trackEvent('login', {
      method: 'github',
      page_location: window.location.href,
      page_referrer: document.referrer,
    })

    const safeContext = {
      page_location: `${window.location.origin}/oauth/github`,
      page_path: '/oauth/github',
      page_referrer: 'https://identity.example/authorize',
    }
    const commands = window.dataLayer as unknown[][]
    const contextIndex = commands.findIndex((command) => command[0] === 'set')
    expect(contextIndex).toBeGreaterThanOrEqual(0)
    expect(contextIndex).toBeLessThan(
      commands.findIndex((command) => command[0] === 'config')
    )
    expect(commands[contextIndex]).toEqual([
      'set',
      expect.objectContaining(safeContext),
    ])
    expect(telemetryEvents().at(-1)).toEqual([
      'event',
      'login',
      expect.objectContaining({ ...safeContext, method: 'github' }),
    ])
    expect(JSON.stringify(commands)).not.toMatch(
      /oauth-secret|state-secret|fragment-secret|referrer-secret|referrer-fragment/
    )
  })

  test('refreshes safe GA4 context for route changes and events before the page view effect', async () => {
    window.__SITE_RUNTIME__ = matchingRuntime()
    const telemetry = await loadTelemetry()
    telemetry.setAnalyticsConsent('granted')
    window.dataLayer = []
    window.history.pushState(
      {},
      '',
      '/oauth/github?code=route-secret#route-fragment'
    )

    telemetry.trackEvent('login', { method: 'github' })
    expect(telemetryEvents().at(-1)).toEqual([
      'event',
      'login',
      expect.objectContaining({
        page_location: `${window.location.origin}/oauth/github`,
        page_referrer: '',
      }),
    ])

    window.history.pushState(
      {},
      '',
      '/wallet?session_id=checkout-secret#checkout-fragment'
    )
    telemetry.trackPageView(
      '/wallet?session_id=checkout-secret#checkout-fragment'
    )
    telemetry.trackEvent('checkout_created', { checkout_type: 'subscription' })

    expect(window.dataLayer).toContainEqual([
      'set',
      expect.objectContaining({
        page_location: `${window.location.origin}/wallet`,
        page_path: '/wallet',
        page_referrer: '',
      }),
    ])
    for (const event of telemetryEvents().slice(1)) {
      expect(event[2]).toEqual(
        expect.objectContaining({
          page_location: `${window.location.origin}/wallet`,
          page_path: '/wallet',
          page_referrer: '',
        })
      )
    }
    expect(JSON.stringify(window.dataLayer)).not.toMatch(
      /route-secret|route-fragment|checkout-secret|checkout-fragment/
    )
  })

  test.each(['', 'not-a-url', 'javascript:referrer-secret'])(
    'clears unusable referrers (%s) instead of falling back to browser defaults',
    async (referrer) => {
      vi.spyOn(document, 'referrer', 'get').mockReturnValue(referrer)
      window.__SITE_RUNTIME__ = matchingRuntime()
      const telemetry = await loadTelemetry()

      telemetry.setAnalyticsConsent('granted')

      expect(telemetryEvents().at(-1)).toEqual([
        'event',
        'page_view',
        expect.objectContaining({ page_referrer: '' }),
      ])
    }
  )

  test('does not load remote tags or emit events before consent', async () => {
    window.__SITE_RUNTIME__ = matchingRuntime()
    const telemetry = await loadTelemetry()

    telemetry.initializeSiteTelemetry()
    telemetry.trackEvent('sign_up', { method: 'password' })

    expect(document.querySelector('#site-telemetry-ga4')).toBeNull()
    expect(document.querySelector('#site-telemetry-clarity')).toBeNull()
    expect(telemetryEvents()).toEqual([])
  })

  test('loads each provider once and de-duplicates page views after consent', async () => {
    window.__SITE_RUNTIME__ = matchingRuntime()
    const telemetry = await loadTelemetry()

    telemetry.setAnalyticsConsent('granted')
    telemetry.trackPageView('/pricing/')
    telemetry.trackPageView('/pricing/')

    expect(
      document.querySelector<HTMLScriptElement>('#site-telemetry-ga4')?.src
    ).toContain('googletagmanager.com/gtag/js?id=G-TEST123')
    expect(
      document.querySelector<HTMLScriptElement>('#site-telemetry-clarity')?.src
    ).toContain('clarity.ms/tag/abc123')
    expect(
      telemetryEvents().filter((event) => event[1] === 'page_view')
    ).toHaveLength(2)
    expect(telemetryEvents().at(-1)).toEqual([
      'event',
      'page_view',
      expect.objectContaining({
        page_location: `${window.location.origin}/pricing/`,
        page_path: '/pricing/',
      }),
    ])
  })

  test('keeps the current consent when local storage is unavailable', async () => {
    const localStorageDescriptor = Object.getOwnPropertyDescriptor(
      window,
      'localStorage'
    )
    Object.defineProperty(window, 'localStorage', {
      configurable: true,
      get: () => {
        throw new Error('storage is unavailable')
      },
    })

    try {
      window.__SITE_RUNTIME__ = matchingRuntime()
      const telemetry = await loadTelemetry()

      telemetry.setAnalyticsConsent('granted')
      telemetry.trackEvent('sign_up', { method: 'password' })

      expect(telemetry.getAnalyticsConsent()).toBe('granted')
      expect(document.querySelector('#site-telemetry-ga4')).not.toBeNull()
      expect(document.querySelector('#site-telemetry-clarity')).not.toBeNull()
      expect(telemetryEvents()).toContainEqual([
        'event',
        'sign_up',
        expect.objectContaining({ method: 'password' }),
      ])
    } finally {
      if (localStorageDescriptor) {
        Object.defineProperty(window, 'localStorage', localStorageDescriptor)
      }
    }
  })

  test('ignores runtime configuration from another origin', async () => {
    window.__SITE_RUNTIME__ = {
      ...matchingRuntime(),
      canonicalOrigin: 'https://tryvalo.com',
      telemetryOrigin: 'https://tryvalo.com',
    }
    const telemetry = await loadTelemetry()

    telemetry.setAnalyticsConsent('granted')
    telemetry.trackEvent('sign_up', { method: 'password' })
    telemetry.updateCanonicalLink('/about/')

    expect(document.querySelector('#site-telemetry-ga4')).toBeNull()
    expect(document.querySelector('#site-telemetry-clarity')).toBeNull()
    expect(telemetryEvents()).toEqual([])
    expect(document.querySelector('link[rel="canonical"]')).toBeNull()
  })

  test('sets a pathname-specific canonical URL without query parameters', async () => {
    window.__SITE_RUNTIME__ = matchingRuntime()
    const telemetry = await loadTelemetry()

    telemetry.updateCanonicalLink('/privacy-policy')

    expect(document.querySelector('link[rel="canonical"]')).toHaveAttribute(
      'href',
      `${window.location.origin}/privacy-policy`
    )
  })
})
