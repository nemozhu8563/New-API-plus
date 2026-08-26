import { describe, expect, test } from 'vitest'

import { isLikelyUntranslated } from '../../../scripts/i18n/untranslated-detection.mjs'

describe('i18n untranslated detection', () => {
  test('flags English UI copy in the Traditional Chinese locale', () => {
    const copy = 'Configure basic system information and branding'

    expect(
      isLikelyUntranslated({ locale: 'zh-TW', baseValue: copy, value: copy })
    ).toBe(true)
  })

  test('keeps registered brands and translated values out of the report', () => {
    expect(
      isLikelyUntranslated({
        locale: 'zh-TW',
        baseValue: 'OpenAI',
        value: 'OpenAI',
      })
    ).toBe(false)
    expect(
      isLikelyUntranslated({
        locale: 'zh-TW',
        baseValue: 'neko-api-key-tool',
        value: 'neko-api-key-tool',
      })
    ).toBe(false)
    expect(
      isLikelyUntranslated({
        locale: 'zh-TW',
        baseValue: 'Doubao Coding Plan',
        value: 'Doubao Coding Plan',
      })
    ).toBe(false)
    expect(
      isLikelyUntranslated({
        locale: 'zh-TW',
        baseValue: 'Sidebar',
        value: '側邊欄',
      })
    ).toBe(false)
  })
})
