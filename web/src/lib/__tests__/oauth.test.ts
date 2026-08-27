import { describe, expect, test } from 'vitest'

import { buildOIDCOAuthUrl } from '../oauth'

describe('buildOIDCOAuthUrl', () => {
  test('uses the configured server address for the OIDC callback', () => {
    const authorizationUrl = buildOIDCOAuthUrl(
      'https://accounts.google.com/o/oauth2/v2/auth',
      'google-client-id',
      'state-token',
      ' https://api.tryvalo.com/ '
    )

    const url = new URL(authorizationUrl)
    expect(url.searchParams.get('redirect_uri')).toBe(
      'https://api.tryvalo.com/api/oauth/oidc'
    )
    expect(url.searchParams.get('client_id')).toBe('google-client-id')
    expect(url.searchParams.get('state')).toBe('state-token')
  })
})
