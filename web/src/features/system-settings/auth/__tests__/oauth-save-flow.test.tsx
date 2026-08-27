import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { SettingsPageProvider } from '@/features/system-settings/components/settings-page-context'

import { OAuthSection } from '../oauth-section'

const mocks = vi.hoisted(() => ({
  mutateAsync: vi.fn(),
}))

vi.mock('../../hooks/use-update-option', () => ({
  useUpdateOption: () => ({
    isPending: false,
    mutateAsync: mocks.mutateAsync,
  }),
}))

vi.mock('../../components/form-navigation-guard', () => ({
  FormNavigationGuard: () => null,
}))

const defaultValues = {
  GitHubOAuthEnabled: false,
  GitHubClientId: '',
  GitHubClientSecret: '',
  'discord.enabled': false,
  'discord.client_id': '',
  'discord.client_secret': '',
  'oidc.enabled': false,
  'oidc.display_name': '',
  'oidc.client_id': '',
  'oidc.client_secret': '',
  'oidc.well_known': '',
  'oidc.authorization_endpoint': '',
  'oidc.token_endpoint': '',
  'oidc.user_info_endpoint': '',
  TelegramOAuthEnabled: false,
  TelegramBotToken: '',
  TelegramBotName: '',
  LinuxDOOAuthEnabled: false,
  LinuxDOClientId: '',
  LinuxDOClientSecret: '',
  LinuxDOMinimumTrustLevel: '',
  WeChatAuthEnabled: false,
  WeChatServerAddress: '',
  WeChatServerToken: '',
  WeChatAccountQRCodeImageURL: '',
}

function renderOAuthSection() {
  const actionsContainer = document.createElement('div')
  document.body.appendChild(actionsContainer)

  render(
    <SettingsPageProvider actionsContainer={actionsContainer}>
      <OAuthSection
        defaultValues={defaultValues}
        serverAddress='https://api.example.com'
      />
    </SettingsPageProvider>
  )
}

describe('OAuth settings save flow', () => {
  beforeEach(() => {
    mocks.mutateAsync.mockReset()
  })

  afterEach(() => {
    document.body.replaceChildren()
  })

  test('saves GitHub credentials before enabling GitHub OAuth', async () => {
    const user = userEvent.setup()
    mocks.mutateAsync.mockResolvedValue({ success: true, message: '' })
    renderOAuthSection()

    fireEvent.change(screen.getByLabelText('Client ID'), {
      target: { value: 'github-client-id' },
    })
    fireEvent.change(screen.getByLabelText('Client Secret'), {
      target: { value: 'github-client-secret' },
    })
    await user.click(screen.getByRole('switch'))
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    await waitFor(() => expect(mocks.mutateAsync).toHaveBeenCalledTimes(3))
    expect(mocks.mutateAsync.mock.calls.map(([request]) => request)).toEqual([
      { key: 'GitHubClientId', value: 'github-client-id' },
      { key: 'GitHubClientSecret', value: 'github-client-secret' },
      { key: 'GitHubOAuthEnabled', value: true },
    ])
  })

  test('stops after a rejected option and retries from the unsaved baseline', async () => {
    const user = userEvent.setup()
    mocks.mutateAsync.mockResolvedValue({
      success: false,
      message: 'configuration rejected',
    })
    renderOAuthSection()

    fireEvent.change(screen.getByLabelText('Client ID'), {
      target: { value: 'github-client-id' },
    })
    fireEvent.change(screen.getByLabelText('Client Secret'), {
      target: { value: 'github-client-secret' },
    })
    await user.click(screen.getByRole('switch'))
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    await waitFor(() => expect(mocks.mutateAsync).toHaveBeenCalledTimes(1))
    expect(mocks.mutateAsync).toHaveBeenLastCalledWith({
      key: 'GitHubClientId',
      value: 'github-client-id',
    })

    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    await waitFor(() => expect(mocks.mutateAsync).toHaveBeenCalledTimes(2))
    expect(mocks.mutateAsync).toHaveBeenLastCalledWith({
      key: 'GitHubClientId',
      value: 'github-client-id',
    })
  })
})
