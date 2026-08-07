import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { resolvePrimaryApiAddress } from '../api-info'

describe('dashboard API address resolution', () => {
  test('uses the status server address instead of the dashboard origin', () => {
    assert.equal(
      resolvePrimaryApiAddress(
        'https://api.tryvalo.com',
        'https://tryvalo.com'
      ),
      'https://api.tryvalo.com/v1'
    )
  })

  test('does not append a duplicate version path', () => {
    assert.equal(
      resolvePrimaryApiAddress(
        'https://api.tryvalo.com/v1/',
        'https://tryvalo.com'
      ),
      'https://api.tryvalo.com/v1'
    )
  })

  test('uses the current origin only when no configured address exists', () => {
    assert.equal(
      resolvePrimaryApiAddress('', 'https://tryvalo.com'),
      'https://tryvalo.com/v1'
    )
  })
})
