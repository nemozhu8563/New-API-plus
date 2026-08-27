import { render, screen, within } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import type { SystemStatus } from '../../types'
import { OAuthProviders } from '../oauth-providers'

vi.mock('../../hooks/use-oauth-login', () => ({
  useOAuthLogin: () => ({
    isLoading: false,
    githubButtonText: '',
    githubButtonDisabled: false,
    handleGitHubLogin: vi.fn(),
    handleDiscordLogin: vi.fn(),
    handleOIDCLogin: vi.fn(),
    handleLinuxDOLogin: vi.fn(),
    handleTelegramLogin: vi.fn(),
    handleCustomOAuthLogin: vi.fn(),
    isTelegramDialogOpen: false,
    isTelegramPending: false,
    handleTelegramAuthorization: vi.fn(),
    setIsTelegramDialogOpen: vi.fn(),
  }),
}))

vi.mock('../telegram-login-dialog', () => ({
  TelegramLoginDialog: () => null,
}))

describe('OAuth provider icons', () => {
  test('shows the Google brand icon when built-in OIDC is named Google', () => {
    const status: SystemStatus = {
      oidc_enabled: true,
      oidc_display_name: ' Google ',
    }

    render(<OAuthProviders status={status} />)

    const button = screen.getByRole('button', { name: 'Continue with Google' })
    expect(within(button).getByTitle('Google')).toBeInTheDocument()
  })

  test('shows the Google brand icon configured for a custom provider', () => {
    const status: SystemStatus = {
      custom_oauth_providers: [
        {
          id: 1,
          name: 'Workspace Google',
          slug: 'workspace-google',
          icon: 'google',
          client_id: 'client-id',
          authorization_endpoint:
            'https://accounts.google.com/o/oauth2/v2/auth',
          scopes: 'openid profile email',
        },
      ],
    }

    render(<OAuthProviders status={status} />)

    const button = screen.getByRole('button', {
      name: 'Continue with Workspace Google',
    })
    expect(within(button).getByTitle('Google')).toBeInTheDocument()
  })
})
