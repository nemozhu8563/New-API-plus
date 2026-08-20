import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  DEFAULT_CONSOLE_ROUTE,
  PUBLIC_HOME_ROUTE,
  resolveLandingPrimaryRoute,
} from '../app-entry-route'

describe('application entry routing', () => {
  test('keeps the public landing page at the root route', () => {
    assert.equal(PUBLIC_HOME_ROUTE, '/')
  })

  test('sends anonymous landing-page visitors to registration', () => {
    assert.equal(resolveLandingPrimaryRoute(false), '/sign-up')
  })

  test('sends authenticated landing-page visitors to the dashboard', () => {
    assert.equal(resolveLandingPrimaryRoute(true), '/dashboard')
    assert.equal(DEFAULT_CONSOLE_ROUTE, '/dashboard/overview')
  })
})
