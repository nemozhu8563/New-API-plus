import assert from 'node:assert/strict'

import { describe, test } from 'vitest'

import { LANDING_NAV_LINKS, resolveLandingNavLinks } from '../landing-nav-links'

describe('landing header navigation', () => {
  test('keeps plans and dashboard entry points available', () => {
    assert.deepEqual(LANDING_NAV_LINKS, [
      { title: 'Plans', href: '#plans' },
      { title: 'Console', href: '/dashboard' },
    ])
  })

  test('adds the server-enabled model square entry with its access rules', () => {
    const pricingLink = {
      title: 'Model Square',
      href: '/pricing',
      requiresAuth: true,
    }

    assert.deepEqual(resolveLandingNavLinks([pricingLink]), [
      ...LANDING_NAV_LINKS,
      pricingLink,
    ])
  })

  test('omits the model square entry when the server does not enable it', () => {
    assert.deepEqual(
      resolveLandingNavLinks([{ title: 'Console', href: '/dashboard' }]),
      LANDING_NAV_LINKS
    )
  })
})
