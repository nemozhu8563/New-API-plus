import { render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { describe, expect, test } from 'vitest'

import { CodexUsageDialog } from '../codex-usage-dialog'

describe('Codex usage dialog localization', () => {
  test('renders the account plan badge in the active locale', async () => {
    const i18n = createInstance()
    await i18n.use(initReactI18next).init({
      lng: 'zh',
      resources: {
        zh: {
          translation: {
            'Free plan': '免费方案',
          },
        },
      },
    })

    render(
      <I18nextProvider i18n={i18n}>
        <CodexUsageDialog
          open
          onOpenChange={() => undefined}
          response={{
            success: true,
            upstream_status: 200,
            data: { plan_type: 'free' },
          }}
        />
      </I18nextProvider>
    )

    expect(screen.getByText('免费方案')).toBeInTheDocument()
    expect(screen.queryByText('Free')).not.toBeInTheDocument()
  })
})
