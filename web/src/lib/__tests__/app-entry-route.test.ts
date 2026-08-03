import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { DEFAULT_CONSOLE_ROUTE, resolveAppEntryRoute } from '../app-entry-route'

describe('application entry routing', () => {
  test('sends anonymous visitors to sign in', () => {
    assert.equal(resolveAppEntryRoute(false), '/sign-in')
  })

  test('sends authenticated visitors directly to the overview dashboard', () => {
    assert.equal(resolveAppEntryRoute(true), DEFAULT_CONSOLE_ROUTE)
    assert.equal(DEFAULT_CONSOLE_ROUTE, '/dashboard/overview')
  })
})
