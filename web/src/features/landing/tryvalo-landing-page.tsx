import { Link } from '@tanstack/react-router'
import {
  ArrowRight,
  Check,
  Code2,
  Gauge,
  Layers3,
  WalletCards,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Footer } from '@/components/layout/components/footer'
import { PublicLayout } from '@/components/layout/components/public-layout'
import type { TopNavLink } from '@/components/layout/types'
import { Button } from '@/components/ui/button'
import {
  PUBLIC_HOME_ROUTE,
  resolveLandingPrimaryRoute,
} from '@/lib/app-entry-route'
import { useAuthStore } from '@/stores/auth-store'

const LANDING_NAV_LINKS: TopNavLink[] = [
  { title: 'Pricing', href: '/pricing' },
  { title: 'About', href: '/about' },
  { title: 'Terms of Service', href: '/user-agreement' },
  { title: 'Privacy Policy', href: '/privacy-policy' },
]

const REQUEST_EXAMPLE = `curl "$TRYVALO_API_BASE/v1/chat/completions" \\
  -H "Authorization: Bearer $TRYVALO_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "your-model",
    "messages": [{"role": "user", "content": "Hello"}]
  }'`

const CAPABILITIES = [
  {
    title: 'One endpoint',
    description:
      'Use a familiar API format and keep your client integration focused.',
    icon: Code2,
  },
  {
    title: 'Model choice',
    description:
      'Choose from available models based on quality, speed, and cost.',
    icon: Layers3,
  },
  {
    title: 'Transparent usage',
    description:
      'Review public model prices before signing up and track usage after login.',
    icon: Gauge,
  },
] as const

const ONBOARDING_STEPS = [
  'Create an account',
  'Create an API key',
  'Send your first request',
] as const

export function TryvaloLandingPage() {
  const { t } = useTranslation()
  const auth = useAuthStore((state) => state.auth)
  const isAuthenticated = Boolean(auth.user && auth.accessToken)
  const primaryRoute = resolveLandingPrimaryRoute(isAuthenticated)

  return (
    <PublicLayout
      showMainContainer={false}
      navLinks={LANDING_NAV_LINKS}
      headerProps={{ homeUrl: PUBLIC_HOME_ROUTE }}
    >
      <main>
        <section className='relative overflow-hidden border-b pt-28 pb-16 sm:pt-36 sm:pb-24'>
          <div
            className='pointer-events-none absolute inset-0 -z-10 [background-image:linear-gradient(to_right,var(--border)_1px,transparent_1px),linear-gradient(to_bottom,var(--border)_1px,transparent_1px)] [mask-image:linear-gradient(to_bottom,black,transparent_85%)] [background-size:40px_40px] opacity-40'
            aria-hidden='true'
          />
          <div className='mx-auto grid max-w-6xl items-center gap-12 px-6 lg:grid-cols-[1.05fr_0.95fr] lg:gap-16'>
            <div>
              <div className='border-border bg-background text-muted-foreground mb-6 inline-flex items-center gap-2 rounded-full border px-3 py-1 text-xs font-medium'>
                <span className='bg-primary size-1.5 rounded-full' />
                {t('Developer API platform')}
              </div>
              <h1 className='max-w-3xl text-4xl leading-[1.05] font-semibold tracking-[-0.04em] text-balance sm:text-6xl'>
                {t('One API for the models your product needs')}
              </h1>
              <p className='text-muted-foreground mt-6 max-w-2xl text-base leading-7 text-pretty sm:text-lg'>
                {t(
                  'Build against an OpenAI-compatible endpoint, choose the model per request, and pay only for actual API usage.'
                )}
              </p>
              <div className='mt-8 flex flex-col gap-3 sm:flex-row'>
                <Button
                  size='lg'
                  className='h-11 rounded-xl px-5'
                  render={<Link to={primaryRoute} />}
                >
                  {isAuthenticated ? t('Open dashboard') : t('Start building')}
                  <ArrowRight data-icon='inline-end' />
                </Button>
                <Button
                  variant='outline'
                  size='lg'
                  className='h-11 rounded-xl px-5'
                  render={<Link to='/pricing' />}
                >
                  {t('View live pricing')}
                </Button>
              </div>
              <div className='text-muted-foreground mt-8 flex flex-wrap gap-x-5 gap-y-2 text-xs'>
                {[
                  'OpenAI-compatible',
                  'Prepaid usage credits',
                  'Usage-based pricing',
                ].map((item) => (
                  <span key={item} className='inline-flex items-center gap-1.5'>
                    <Check className='text-success size-3.5' />
                    {t(item)}
                  </span>
                ))}
              </div>
            </div>

            <div className='ring-foreground/10 bg-card overflow-hidden rounded-2xl ring-1'>
              <div className='border-border flex items-center justify-between border-b px-4 py-3'>
                <span className='text-sm font-medium'>
                  {t('API request example')}
                </span>
                <span className='text-muted-foreground font-mono text-[11px]'>
                  POST /v1/chat/completions
                </span>
              </div>
              <pre className='text-card-foreground overflow-x-auto p-5 font-mono text-xs leading-6 sm:p-6'>
                <code>{REQUEST_EXAMPLE}</code>
              </pre>
              <div className='bg-muted/40 border-border text-muted-foreground border-t px-4 py-3 text-xs'>
                {t('Switch models without rebuilding your integration')}
              </div>
            </div>
          </div>
        </section>

        <section className='mx-auto max-w-6xl px-6 py-16 sm:py-24'>
          <div className='grid gap-4 md:grid-cols-3'>
            {CAPABILITIES.map((capability) => {
              const Icon = capability.icon
              return (
                <article
                  key={capability.title}
                  className='ring-foreground/10 bg-card rounded-2xl p-5 ring-1'
                >
                  <div className='bg-accent text-accent-foreground flex size-9 items-center justify-center rounded-xl'>
                    <Icon className='size-4' />
                  </div>
                  <h2 className='mt-5 text-base font-medium'>
                    {t(capability.title)}
                  </h2>
                  <p className='text-muted-foreground mt-2 text-sm leading-6 text-pretty'>
                    {t(capability.description)}
                  </p>
                </article>
              )
            })}
          </div>
        </section>

        <section className='bg-muted/30 border-y'>
          <div className='mx-auto grid max-w-6xl gap-10 px-6 py-16 sm:py-24 lg:grid-cols-[0.8fr_1.2fr] lg:items-start'>
            <div>
              <p className='text-primary text-xs font-semibold tracking-[0.16em] uppercase'>
                {t('How it works')}
              </p>
              <h2 className='mt-3 text-3xl font-semibold tracking-[-0.03em] text-balance'>
                {t('From account to first request in three clear steps.')}
              </h2>
            </div>
            <ol className='grid gap-3'>
              {ONBOARDING_STEPS.map((step, index) => (
                <li
                  key={step}
                  className='bg-background ring-foreground/10 flex items-center gap-4 rounded-2xl p-4 ring-1'
                >
                  <span className='bg-foreground text-background flex size-8 shrink-0 items-center justify-center rounded-full font-mono text-xs'>
                    {String(index + 1).padStart(2, '0')}
                  </span>
                  <span className='text-sm font-medium'>{t(step)}</span>
                </li>
              ))}
            </ol>
          </div>
        </section>

        <section className='mx-auto max-w-6xl px-6 py-16 sm:py-24'>
          <div className='ring-foreground/10 bg-card grid overflow-hidden rounded-3xl ring-1 lg:grid-cols-[1fr_auto]'>
            <div className='p-6 sm:p-10'>
              <div className='bg-accent text-accent-foreground flex size-10 items-center justify-center rounded-xl'>
                <WalletCards className='size-5' />
              </div>
              <h2 className='mt-6 text-2xl font-semibold tracking-[-0.025em]'>
                {t('Pricing that follows usage')}
              </h2>
              <p className='text-muted-foreground mt-3 max-w-2xl text-sm leading-6 text-pretty'>
                {t(
                  'No invented SaaS tiers. Review the current per-model rates and fund your account when needed.'
                )}
              </p>
            </div>
            <div className='bg-muted/35 border-t p-6 lg:flex lg:min-w-72 lg:items-center lg:border-t-0 lg:border-l lg:p-10'>
              <Button
                variant='outline'
                size='lg'
                className='h-11 w-full rounded-xl px-5'
                render={<Link to='/pricing' />}
              >
                {t('Review model pricing')}
                <ArrowRight data-icon='inline-end' />
              </Button>
            </div>
          </div>
        </section>

        <section className='border-t'>
          <div className='mx-auto flex max-w-6xl flex-col items-start justify-between gap-6 px-6 py-16 sm:flex-row sm:items-center sm:py-20'>
            <div>
              <h2 className='text-2xl font-semibold tracking-[-0.025em]'>
                {t('Ready to make your first API call?')}
              </h2>
              <p className='text-muted-foreground mt-2 max-w-2xl text-sm leading-6'>
                {t(
                  'Create an account, add credit, and use the same API format across supported models.'
                )}
              </p>
            </div>
            <Button
              size='lg'
              className='h-11 rounded-xl px-5'
              render={<Link to={primaryRoute} />}
            >
              {isAuthenticated ? t('Open dashboard') : t('Start building')}
              <ArrowRight data-icon='inline-end' />
            </Button>
          </div>
        </section>
      </main>

      <Footer homeUrl={PUBLIC_HOME_ROUTE} />
    </PublicLayout>
  )
}
