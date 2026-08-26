import assert from 'node:assert/strict'

import { describe, test } from 'vitest'

import { resolveTopNavLinks } from '../top-nav.config'

describe('public header navigation', () => {
  test('keeps explicitly provided legal links when dynamic navigation is available', () => {
    const providedLinks = [
      { title: 'Terms of Service', href: '/user-agreement' },
      { title: 'Privacy Policy', href: '/privacy-policy' },
    ]
    const dynamicLinks = [{ title: 'Console', href: '/dashboard' }]

    assert.deepEqual(
      resolveTopNavLinks(providedLinks, dynamicLinks),
      providedLinks
    )
  })

  test('uses dynamic navigation when no links are explicitly provided', () => {
    const dynamicLinks = [{ title: 'Console', href: '/dashboard' }]

    assert.deepEqual(resolveTopNavLinks(undefined, dynamicLinks), dynamicLinks)
  })
})
