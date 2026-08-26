import assert from 'node:assert/strict'

import { describe, test } from 'vitest'

import { walletLayoutClasses } from '../layout'

describe('wallet layout', () => {
  test('keeps recharge and subscription sections in one top-to-bottom flow', () => {
    const classes = walletLayoutClasses.primarySections.split(' ')

    assert.ok(classes.includes('flex'))
    assert.ok(classes.includes('flex-col'))
    assert.equal(
      classes.some((className) => className.includes('grid')),
      false
    )
  })

  test('keeps wide wallet forms at a readable maximum width', () => {
    const classes = walletLayoutClasses.content.split(' ')

    assert.ok(classes.includes('max-w-5xl'))
  })
})
