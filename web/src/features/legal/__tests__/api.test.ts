import assert from 'node:assert/strict'
import { afterEach, describe, test } from 'node:test'

import { api } from '@/lib/api'

import { getPrivacyPolicy, getUserAgreement } from '../api'

const originalAdapter = api.defaults.adapter

afterEach(() => {
  api.defaults.adapter = originalAdapter
})

describe('legal document API', () => {
  test('sends the active interface locale for each public legal document', async () => {
    const requests: Array<{ url: string; locale: unknown }> = []
    api.defaults.adapter = async (config) => {
      requests.push({
        url: config.url || '',
        locale: config.params?.locale,
      })
      return {
        data: { success: true, data: 'document' },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }

    await getUserAgreement('zhTW')
    await getPrivacyPolicy('fr')

    assert.deepEqual(requests, [
      { url: '/api/user-agreement', locale: 'zhTW' },
      { url: '/api/privacy-policy', locale: 'fr' },
    ])
  })
})
