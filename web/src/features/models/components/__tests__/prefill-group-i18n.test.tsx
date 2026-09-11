import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { describe, expect, test } from 'vitest'

import { PrefillGroupFormDrawer } from '../drawers/prefill-group-form-drawer'

describe('prefill group localization', () => {
  test('renders the group type field and selected type in the active locale', async () => {
    const i18n = createInstance()
    await i18n.use(initReactI18next).init({
      lng: 'zh',
      resources: {
        zh: {
          translation: {
            'Group Type': '分组类型',
            'Model Group': '模型组',
            'Reusable sets of models you can attach to channels.':
              '可附加到渠道的可复用模型集合。',
          },
        },
      },
    })
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    render(
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={queryClient}>
          <PrefillGroupFormDrawer
            open
            onClose={() => undefined}
            currentGroup={null}
          />
        </QueryClientProvider>
      </I18nextProvider>
    )

    expect(screen.getByText('分组类型')).toBeInTheDocument()
    expect(screen.getByRole('combobox')).toHaveTextContent('模型组')
  })
})
