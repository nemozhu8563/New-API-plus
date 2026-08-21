import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { LANDING_NAV_LINKS } from '../landing-nav-links'

describe('landing header navigation', () => {
  test('keeps plans and dashboard entry points available', () => {
    assert.deepEqual(LANDING_NAV_LINKS, [
      { title: 'Plans', href: '#plans' },
      { title: 'Console', href: '/dashboard' },
    ])
  })
})
