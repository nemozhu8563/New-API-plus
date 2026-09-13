import type { TopNavLink } from '@/components/layout/types'

export const LANDING_NAV_LINKS: TopNavLink[] = [
  { title: 'Plans', href: '#plans' },
  { title: 'Console', href: '/dashboard' },
]

/**
 * Keep the landing page's bespoke links while honoring the server-controlled
 * model square visibility and authentication requirement.
 */
export function resolveLandingNavLinks(
  dynamicLinks: TopNavLink[]
): TopNavLink[] {
  const pricingLink = dynamicLinks.find((link) => link.href === '/pricing')
  if (!pricingLink) return LANDING_NAV_LINKS

  return [...LANDING_NAV_LINKS, pricingLink]
}
