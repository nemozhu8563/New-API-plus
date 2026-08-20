import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { getModuleAccessFromStatus } from '../nav-modules'

describe('header navigation module defaults', () => {
  test('keeps model pricing behind authentication by default', () => {
    assert.deepEqual(getModuleAccessFromStatus(null, 'pricing'), {
      enabled: true,
      requireAuth: true,
    })
  })
})
