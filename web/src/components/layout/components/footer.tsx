import { Link } from '@tanstack/react-router'
import { Fragment, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { useStatus } from '@/hooks/use-status'
import { useSystemConfig } from '@/hooks/use-system-config'
import { cn } from '@/lib/utils'

interface FooterLink {
  text: string
  href: string
}

interface FooterColumnProps {
  title: string
  links: FooterLink[]
}

interface FooterProps {
  logo?: string
  name?: string
  columns?: FooterColumnProps[]
  copyright?: string
  homeUrl?: '/' | '/sign-in'
  className?: string
  variant?: 'default' | 'warm'
}

function FooterLinkItem(props: { link: FooterLink; warm?: boolean }) {
  const { t } = useTranslation()
  const isExternal = props.link.href.startsWith('http')
  const isEmail = props.link.href.startsWith('mailto:')
  const isPageAnchor = props.link.href.startsWith('#')
  const label = t(props.link.text)

  if (isExternal || isEmail || isPageAnchor) {
    return (
      <a
        href={props.link.href}
        target={isExternal ? '_blank' : undefined}
        rel={isExternal ? 'noopener noreferrer' : undefined}
        className={cn(
          'text-sm transition-colors duration-200',
          props.warm
            ? 'text-[#21160f]/70 hover:text-[#21160f]'
            : 'text-muted-foreground hover:text-foreground'
        )}
      >
        {label}
      </a>
    )
  }

  return (
    <Link
      to={props.link.href}
      className={cn(
        'text-sm transition-colors duration-200',
        props.warm
          ? 'text-[#21160f]/70 hover:text-[#21160f]'
          : 'text-muted-foreground hover:text-foreground'
      )}
    >
      {label}
    </Link>
  )
}

// Renders User Agreement / Privacy Policy links inline with the parent's
// copyright row when either is configured in System Settings → Site. Emits
// fragmented siblings so the parent flex container's gap controls spacing.
function LegalLinks(props: { leadingSeparator?: boolean }) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const items: { key: string; label: string; href: string }[] = []
  if (status?.user_agreement_enabled) {
    items.push({
      key: 'user-agreement',
      label: t('Terms of Service'),
      href: '/user-agreement',
    })
  }
  if (status?.privacy_policy_enabled) {
    items.push({
      key: 'privacy-policy',
      label: t('Privacy Policy'),
      href: '/privacy-policy',
    })
  }
  if (items.length === 0) {
    return null
  }
  return (
    <>
      {items.map((item, index) => (
        <Fragment key={item.key}>
          {(props.leadingSeparator || index > 0) && (
            <span aria-hidden='true' className='text-muted-foreground/30'>
              ·
            </span>
          )}
          <Link
            to={item.href}
            className='hover:text-foreground transition-colors duration-200'
          >
            {item.label}
          </Link>
        </Fragment>
      ))}
    </>
  )
}

export function Footer(props: FooterProps) {
  const { t } = useTranslation()
  const {
    systemName,
    logo: systemLogo,
    footerHtml,
    demoSiteEnabled,
  } = useSystemConfig()

  const displayLogo = systemLogo || props.logo || '/logo.png'
  const displayName = systemName || props.name || 'New API'
  const isDemoSiteMode = Boolean(demoSiteEnabled)
  const currentYear = new Date().getFullYear()

  const fallbackColumns = useMemo<FooterColumnProps[]>(
    () => [
      {
        title: t('footer.columns.about.title'),
        links: [
          {
            text: t('footer.columns.about.links.aboutProject'),
            href: 'https://docs.newapi.pro/wiki/project-introduction/',
          },
          {
            text: t('footer.columns.about.links.contact'),
            href: 'mailto:contract@tryvalo.com',
          },
          {
            text: t('footer.columns.about.links.features'),
            href: 'https://docs.newapi.pro/wiki/features-introduction/',
          },
        ],
      },
      {
        title: t('footer.columns.docs.title'),
        links: [
          {
            text: t('footer.columns.docs.links.quickStart'),
            href: 'https://docs.newapi.pro/getting-started/',
          },
          {
            text: t('footer.columns.docs.links.installation'),
            href: 'https://docs.newapi.pro/installation/',
          },
          {
            text: t('footer.columns.docs.links.apiDocs'),
            href: 'https://docs.newapi.pro/api/',
          },
        ],
      },
    ],
    [t]
  )

  const displayColumns = props.columns ?? fallbackColumns
  const showColumns = Boolean(props.columns?.length) || isDemoSiteMode
  const isWarm = props.variant === 'warm'

  if (footerHtml && !isWarm) {
    return (
      <footer
        className={cn(
          'border-border/40 relative z-10 border-t',
          props.className
        )}
      >
        <div className='mx-auto w-full max-w-6xl px-6 py-5'>
          <div className='bg-muted/20 border-border/50 flex flex-col items-center justify-between gap-4 rounded-2xl border px-4 py-4 backdrop-blur-sm sm:flex-row sm:px-5'>
            <div
              className='custom-footer text-muted-foreground min-w-0 text-center text-sm sm:text-left'
              dangerouslySetInnerHTML={{ __html: footerHtml }}
            />
            <div className='border-border/60 text-muted-foreground/45 flex w-full flex-wrap items-center justify-center gap-x-3 gap-y-1 border-t pt-4 text-xs empty:hidden sm:w-auto sm:justify-end sm:border-t-0 sm:border-l sm:pt-0 sm:pl-5'>
              <LegalLinks />
            </div>
          </div>
        </div>
      </footer>
    )
  }

  return (
    <footer
      className={cn(
        'relative z-10 border-t',
        isWarm
          ? 'border-[#7d472c]/20 bg-[#ef884c] text-[#21160f]'
          : 'border-border/40',
        props.className
      )}
    >
      <div className='mx-auto max-w-6xl px-6 py-12 md:py-16'>
        <div className='flex flex-col justify-between gap-10 md:flex-row md:gap-16'>
          {/* Brand column */}
          <div className='shrink-0'>
            <Link
              to={props.homeUrl ?? '/sign-in'}
              className='group flex items-center gap-2.5'
            >
              <img
                src={displayLogo}
                alt={displayName}
                className='size-7 rounded-lg object-contain'
              />
              <span className='text-sm font-semibold tracking-tight'>
                {displayName}
              </span>
            </Link>
            <p
              className={cn(
                'mt-3 max-w-[200px] text-xs leading-relaxed',
                isWarm ? 'text-[#21160f]/65' : 'text-muted-foreground/60'
              )}
            >
              {t('Powerful API Management Platform')}
            </p>
            {isWarm && (
              <a
                href='mailto:contract@tryvalo.com'
                className='mt-4 block text-sm text-[#21160f]/75 transition-colors hover:text-[#21160f]'
              >
                contract@tryvalo.com
              </a>
            )}
          </div>

          {/* Links columns */}
          {showColumns && (
            <div className='grid grid-cols-2 gap-8 sm:grid-cols-3 md:gap-16'>
              {displayColumns.map((column) => (
                <div key={column.title}>
                  <p
                    className={cn(
                      'mb-3 text-xs font-medium tracking-wider uppercase',
                      isWarm ? 'text-[#21160f]/85' : 'text-muted-foreground/50'
                    )}
                  >
                    {t(column.title)}
                  </p>
                  <ul className='space-y-2.5'>
                    {column.links.map((link) => (
                      <li key={`${link.text}-${link.href}`}>
                        <FooterLinkItem link={link} warm={isWarm} />
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
            </div>
          )}
        </div>

        <div
          className={cn(
            'mt-12 flex flex-col items-center gap-x-3 gap-y-2 border-t pt-6 sm:flex-row',
            isWarm ? 'border-[#7d472c]/20' : 'border-border/30'
          )}
        >
          <div
            className={cn(
              'flex flex-wrap items-center justify-center gap-x-2 gap-y-1 text-xs sm:justify-start',
              isWarm ? 'text-[#21160f]/60' : 'text-muted-foreground/40'
            )}
          >
            <span>
              &copy; {currentYear} {displayName}.{' '}
              {props.copyright ?? t('footer.defaultCopyright')}
            </span>
            {!isWarm && <LegalLinks leadingSeparator />}
          </div>
        </div>
      </div>
    </footer>
  )
}
