import type { TopNavLink } from '../types'

/**
 * Default top navigation links
 *
 * In practice, navigation links are dynamically fetched from backend.
 * Explicit links let a public page preserve required navigation such as legal
 * documents. Otherwise, backend navigation remains the normal source.
 *
 * This is intentionally empty to encourage backend configuration.
 * If you need fallback links, add them here.
 */
export const defaultTopNavLinks: TopNavLink[] = []

export function resolveTopNavLinks(
  providedLinks: TopNavLink[] | undefined,
  dynamicLinks: TopNavLink[]
): TopNavLink[] {
  if (providedLinks) return providedLinks
  if (dynamicLinks.length > 0) return dynamicLinks
  return defaultTopNavLinks
}
