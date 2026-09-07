export type AnalyticsConsent = 'granted' | 'denied'

export type SiteTelemetryEventName =
  | 'sign_up'
  | 'login'
  | 'api_key_created'
  | 'model_request_started'
  | 'model_request_succeeded'
  | 'checkout_created'

type SiteRuntimeConfig = {
  canonicalOrigin?: string
  telemetryOrigin?: string
  googleAnalyticsId?: string
  clarityProjectId?: string
}

type TelemetryParameterValue = string | number | boolean
type TelemetryParameters = Record<string, TelemetryParameterValue | undefined>
type Gtag = (...args: unknown[]) => void
type Clarity = (...args: unknown[]) => void

declare global {
  interface Window {
    __SITE_RUNTIME__?: SiteRuntimeConfig
    __SITE_TELEMETRY_GA_CONSENT_DEFAULTED__?: boolean
    dataLayer?: unknown[]
    gtag?: Gtag
    clarity?: Clarity & { q?: unknown[][] }
  }
}

const CONSENT_STORAGE_KEY = 'tryvalo.analytics-consent.v1'
const GA4_SCRIPT_ID = 'site-telemetry-ga4'
const CLARITY_SCRIPT_ID = 'site-telemetry-clarity'
const MAX_PARAMETER_VALUE_LENGTH = 100
const GA_MEASUREMENT_ID_PATTERN = /^G-[A-Z0-9]+$/
const CLARITY_PROJECT_ID_PATTERN = /^[a-z0-9]+$/
const blockedParameterName =
  /email|name|prompt|message|content|token|key|url|user|session|password|file/i

let gaConfigured = false
let clarityConfigured = false
let lastPageViewPath: string | null = null
let inMemoryConsent: AnalyticsConsent | null = null

function getRuntimeConfig(): SiteRuntimeConfig | undefined {
  if (typeof window === 'undefined') return undefined
  return window.__SITE_RUNTIME__
}

function isExactOrigin(expectedOrigin: string | undefined): boolean {
  return Boolean(expectedOrigin && window.location.origin === expectedOrigin)
}

function hasValidGA4ID(config: SiteRuntimeConfig): boolean {
  return Boolean(
    config.googleAnalyticsId &&
    GA_MEASUREMENT_ID_PATTERN.test(config.googleAnalyticsId)
  )
}

function hasValidClarityID(config: SiteRuntimeConfig): boolean {
  return Boolean(
    config.clarityProjectId &&
    CLARITY_PROJECT_ID_PATTERN.test(config.clarityProjectId)
  )
}

function getTelemetryConfig(): SiteRuntimeConfig | undefined {
  const config = getRuntimeConfig()
  if (!config || !isExactOrigin(config.telemetryOrigin)) return undefined
  if (!hasValidGA4ID(config) && !hasValidClarityID(config)) return undefined
  return config
}

function ensureGtag(): Gtag {
  window.dataLayer ??= []
  window.gtag ??= (...args: unknown[]) => {
    window.dataLayer?.push(args)
  }
  if (!window.__SITE_TELEMETRY_GA_CONSENT_DEFAULTED__) {
    window.gtag('consent', 'default', {
      analytics_storage: 'denied',
      ad_storage: 'denied',
      ad_user_data: 'denied',
      ad_personalization: 'denied',
    })
    window.__SITE_TELEMETRY_GA_CONSENT_DEFAULTED__ = true
  }
  return window.gtag
}

function ensureClarityQueue(): Clarity {
  window.clarity ??= (...args: unknown[]) => {
    const queue = window.clarity?.q ?? []
    queue.push(args)
    if (window.clarity) window.clarity.q = queue
  }
  return window.clarity
}

function updateGoogleConsent(consent: AnalyticsConsent): void {
  const gtag = ensureGtag()
  gtag('consent', 'update', {
    analytics_storage: consent,
    ad_storage: 'denied',
    ad_user_data: 'denied',
    ad_personalization: 'denied',
  })
}

function updateClarityConsent(consent: AnalyticsConsent): void {
  const clarity = ensureClarityQueue()
  clarity('consentv2', {
    ad_Storage: 'denied',
    analytics_Storage: consent,
  })
}

function appendScript(id: string, source: string): void {
  if (document.querySelector(`#${id}`)) return
  const script = document.createElement('script')
  script.id = id
  script.async = true
  script.src = source
  script.referrerPolicy = 'strict-origin-when-cross-origin'
  document.head.appendChild(script)
}

function loadGA4(config: SiteRuntimeConfig): void {
  if (!config.googleAnalyticsId || gaConfigured) return
  const gtag = ensureGtag()
  appendScript(
    GA4_SCRIPT_ID,
    `https://www.googletagmanager.com/gtag/js?id=${encodeURIComponent(config.googleAnalyticsId)}`
  )
  gtag('js', new Date())
  gtag('config', config.googleAnalyticsId, {
    send_page_view: false,
    allow_google_signals: false,
  })
  gaConfigured = true
}

function loadClarity(config: SiteRuntimeConfig): void {
  if (!config.clarityProjectId || clarityConfigured) return
  appendScript(
    CLARITY_SCRIPT_ID,
    `https://www.clarity.ms/tag/${encodeURIComponent(config.clarityProjectId)}`
  )
  clarityConfigured = true
}

function sanitizeParameters(
  parameters: TelemetryParameters
): Record<string, TelemetryParameterValue> {
  const sanitized: Record<string, TelemetryParameterValue> = {}
  for (const [name, value] of Object.entries(parameters)) {
    if (
      value === undefined ||
      blockedParameterName.test(name) ||
      !/^[a-z][a-z0-9_]{0,39}$/.test(name)
    ) {
      continue
    }
    if (typeof value === 'number') {
      if (Number.isFinite(value)) sanitized[name] = Math.round(value)
      continue
    }
    sanitized[name] =
      typeof value === 'string'
        ? value.slice(0, MAX_PARAMETER_VALUE_LENGTH)
        : value
  }
  return sanitized
}

function currentPathname(): string {
  return window.location.pathname || '/'
}

function safeGA4PageContext(
  pathname = currentPathname()
): Record<string, string> {
  const pathWithoutQuery = pathname.split(/[?#]/, 1)[0] || '/'
  const pagePath = pathWithoutQuery.startsWith('/')
    ? pathWithoutQuery
    : `/${pathWithoutQuery}`
  let pageReferrer = ''
  try {
    const referrer = new URL(document.referrer)
    if (referrer.protocol === 'https:' || referrer.protocol === 'http:') {
      pageReferrer = `${referrer.origin}${referrer.pathname}`
    }
  } catch {
    // Explicitly clear invalid/empty referrers instead of using GA4's raw default.
  }
  return {
    page_location: `${window.location.origin}${pagePath}`,
    page_path: pagePath,
    page_referrer: pageReferrer,
    page_title: document.title,
  }
}

function amountBucket(amount: number): string {
  if (!Number.isFinite(amount) || amount <= 0) return 'unknown'
  if (amount < 20) return 'under_20'
  if (amount < 100) return '20_99'
  if (amount < 500) return '100_499'
  if (amount < 1000) return '500_999'
  return '1000_plus'
}

export function isSiteTelemetryEnabled(): boolean {
  return Boolean(getTelemetryConfig())
}

export function getAnalyticsConsent(): AnalyticsConsent | null {
  if (typeof window === 'undefined') return null
  if (inMemoryConsent) return inMemoryConsent
  try {
    const value = window.localStorage.getItem(CONSENT_STORAGE_KEY)
    return value === 'granted' || value === 'denied' ? value : null
  } catch {
    return null
  }
}

export function initializeSiteTelemetry(): void {
  const config = getTelemetryConfig()
  if (!config) return

  const consent = getAnalyticsConsent() ?? 'denied'
  if (hasValidGA4ID(config)) {
    // Set safe defaults before config/consent can trigger provider events.
    ensureGtag()('set', safeGA4PageContext())
    updateGoogleConsent(consent)
  }
  if (hasValidClarityID(config)) updateClarityConsent(consent)
  if (consent !== 'granted') return

  if (hasValidGA4ID(config)) loadGA4(config)
  if (hasValidClarityID(config)) loadClarity(config)
}

export function setAnalyticsConsent(consent: AnalyticsConsent): void {
  if (!isSiteTelemetryEnabled()) return
  inMemoryConsent = consent
  try {
    window.localStorage.setItem(CONSENT_STORAGE_KEY, consent)
  } catch {
    // Storage can be disabled; apply the choice for this page session anyway.
  }

  lastPageViewPath = null
  initializeSiteTelemetry()
  if (consent === 'granted') trackPageView(currentPathname())
}

export function trackEvent(
  eventName: SiteTelemetryEventName,
  parameters: TelemetryParameters = {}
): void {
  const config = getTelemetryConfig()
  if (
    !config ||
    getAnalyticsConsent() !== 'granted' ||
    !hasValidGA4ID(config)
  ) {
    return
  }

  initializeSiteTelemetry()
  ensureGtag()('event', eventName, {
    ...sanitizeParameters(parameters),
    ...safeGA4PageContext(),
  })
}

export function trackPageView(pathname: string): void {
  const config = getTelemetryConfig()
  if (
    !config ||
    getAnalyticsConsent() !== 'granted' ||
    !hasValidGA4ID(config)
  ) {
    return
  }

  const pageContext = safeGA4PageContext(pathname)
  if (lastPageViewPath === pageContext.page_path) return
  lastPageViewPath = pageContext.page_path
  initializeSiteTelemetry()
  const gtag = ensureGtag()
  gtag('set', pageContext)
  gtag('event', 'page_view', pageContext)
}

export function updateCanonicalLink(pathname: string): void {
  const config = getRuntimeConfig()
  if (!config?.canonicalOrigin || !isExactOrigin(config.canonicalOrigin)) return

  const normalizedPath = pathname.startsWith('/') ? pathname : `/${pathname}`
  let canonical = document.querySelector<HTMLLinkElement>(
    'link[rel="canonical"]'
  )
  if (!canonical) {
    canonical = document.createElement('link')
    canonical.rel = 'canonical'
    document.head.appendChild(canonical)
  }
  canonical.href = `${config.canonicalOrigin}${normalizedPath}`
}

export { amountBucket }
