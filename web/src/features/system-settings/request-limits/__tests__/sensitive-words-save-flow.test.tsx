import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { SettingsPageProvider } from '@/features/system-settings/components/settings-page-context'

import { SensitiveWordsSection } from '../sensitive-words-section'

const mocks = vi.hoisted(() => ({
  mutateAsync: vi.fn(),
}))

vi.mock('../../hooks/use-update-option', () => ({
  useUpdateOption: () => ({
    isPending: false,
    mutateAsync: mocks.mutateAsync,
  }),
}))

const defaultValues = {
  CheckSensitiveEnabled: true,
  CheckSensitiveOnPromptEnabled: true,
  SensitiveWordsHighRisk: 'existing-risk',
  SensitiveWordsAudit: 'existing-audit',
  SensitiveWords: 'existing-nsfw',
}

function renderSensitiveWordsSection() {
  const actionsContainer = document.createElement('div')
  document.body.appendChild(actionsContainer)

  render(
    <SettingsPageProvider actionsContainer={actionsContainer}>
      <SensitiveWordsSection defaultValues={defaultValues} />
    </SettingsPageProvider>
  )
}

describe('Sensitive word settings save flow', () => {
  beforeEach(() => {
    mocks.mutateAsync.mockReset()
    mocks.mutateAsync.mockResolvedValue({ success: true, message: '' })
  })

  afterEach(() => {
    document.body.replaceChildren()
  })

  test('saves high-risk and audit lists before replacing the NSFW list', async () => {
    const user = userEvent.setup()
    renderSensitiveWordsSection()

    fireEvent.change(screen.getByLabelText('High-risk blocked keywords'), {
      target: { value: 'updated-risk' },
    })
    fireEvent.change(screen.getByLabelText('Audit-only keywords'), {
      target: { value: 'updated-audit' },
    })
    fireEvent.change(screen.getByLabelText('NSFW blocked keywords'), {
      target: { value: 'updated-nsfw' },
    })
    for (const toggle of screen.getAllByRole('switch')) {
      await user.click(toggle)
    }
    await user.click(
      screen.getByRole('button', { name: 'Save sensitive words' })
    )

    await waitFor(() => expect(mocks.mutateAsync).toHaveBeenCalledTimes(5))
    expect(mocks.mutateAsync.mock.calls.map(([request]) => request)).toEqual([
      { key: 'SensitiveWordsHighRisk', value: 'updated-risk' },
      { key: 'SensitiveWordsAudit', value: 'updated-audit' },
      { key: 'SensitiveWords', value: 'updated-nsfw' },
      { key: 'CheckSensitiveOnPromptEnabled', value: false },
      { key: 'CheckSensitiveEnabled', value: false },
    ])
  })

  test('does not send unchanged word lists', async () => {
    const user = userEvent.setup()
    renderSensitiveWordsSection()

    fireEvent.change(screen.getByLabelText('Audit-only keywords'), {
      target: { value: 'updated-audit' },
    })
    await user.click(
      screen.getByRole('button', { name: 'Save sensitive words' })
    )

    await waitFor(() => expect(mocks.mutateAsync).toHaveBeenCalledTimes(1))
    expect(mocks.mutateAsync).toHaveBeenCalledWith({
      key: 'SensitiveWordsAudit',
      value: 'updated-audit',
    })
  })

  test('stops before changing switches when a word-list update fails', async () => {
    const user = userEvent.setup()
    mocks.mutateAsync.mockImplementation(async ({ key }) => ({
      success: key !== 'SensitiveWords',
      message: key === 'SensitiveWords' ? 'configuration rejected' : '',
    }))
    renderSensitiveWordsSection()

    fireEvent.change(screen.getByLabelText('High-risk blocked keywords'), {
      target: { value: 'updated-risk' },
    })
    fireEvent.change(screen.getByLabelText('Audit-only keywords'), {
      target: { value: 'updated-audit' },
    })
    fireEvent.change(screen.getByLabelText('NSFW blocked keywords'), {
      target: { value: 'updated-nsfw' },
    })
    for (const toggle of screen.getAllByRole('switch')) {
      await user.click(toggle)
    }
    await user.click(
      screen.getByRole('button', { name: 'Save sensitive words' })
    )

    await waitFor(() => expect(mocks.mutateAsync).toHaveBeenCalledTimes(3))
    expect(mocks.mutateAsync.mock.calls.map(([request]) => request)).toEqual([
      { key: 'SensitiveWordsHighRisk', value: 'updated-risk' },
      { key: 'SensitiveWordsAudit', value: 'updated-audit' },
      { key: 'SensitiveWords', value: 'updated-nsfw' },
    ])
  })
})
