import { act, render } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { describe, expect, test } from 'vitest'

import en from '@/i18n/locales/en.json'
import zh from '@/i18n/locales/zh.json'

import { LegalConsent } from '../legal-consent'
import { TermsFooter } from '../terms-footer'

const status = {
  user_agreement_enabled: true,
  privacy_policy_enabled: true,
}

async function renderInChinese(ui: React.ReactNode) {
  const i18n = createInstance()
  await i18n.use(initReactI18next).init({
    lng: 'zh',
    fallbackLng: 'zh',
    resources: { en, zh },
  })

  return {
    i18n,
    ...render(<I18nextProvider i18n={i18n}>{ui}</I18nextProvider>),
  }
}

describe('authentication legal copy localization', () => {
  test('renders sign-in consent copy entirely in Chinese', async () => {
    const consent = await renderInChinese(
      <LegalConsent
        status={status}
        checked={false}
        onCheckedChange={() => undefined}
      />
    )

    expect(consent.container).toHaveTextContent(
      '我已阅读并同意服务协议和隐私政策。'
    )
    expect(consent.getByRole('link', { name: '服务协议' })).toHaveAttribute(
      'href',
      '/user-agreement'
    )
    expect(consent.getByRole('link', { name: '隐私政策' })).toHaveAttribute(
      'href',
      '/privacy-policy'
    )

    consent.unmount()

    const footer = await renderInChinese(
      <TermsFooter variant='sign-in' status={status} />
    )

    expect(footer.container).toHaveTextContent(
      '点击登录，即表示您同意我们的服务协议和隐私政策。'
    )
    expect(footer.getByRole('link', { name: '服务协议' })).toHaveAttribute(
      'href',
      '/user-agreement'
    )
    expect(footer.getByRole('link', { name: '隐私政策' })).toHaveAttribute(
      'href',
      '/privacy-policy'
    )
  })

  test('renders sign-up policy copy entirely in Chinese', async () => {
    const footer = await renderInChinese(
      <TermsFooter variant='sign-up' status={status} />
    )

    expect(footer.container).toHaveTextContent(
      '创建账户即表示您同意我们的服务协议和隐私政策。'
    )
  })

  test('updates policy copy when the interface language changes', async () => {
    const view = await renderInChinese(
      <TermsFooter variant='sign-in' status={status} />
    )

    await act(async () => {
      await view.i18n.changeLanguage('en')
    })

    expect(view.container).toHaveTextContent(
      'By clicking sign in, you agree to our Terms of Service and Privacy Policy.'
    )
  })

  test('mentions only the terms when the privacy policy is disabled', async () => {
    const view = await renderInChinese(
      <>
        <LegalConsent
          status={{ user_agreement_enabled: true }}
          checked={false}
          onCheckedChange={() => undefined}
        />
        <TermsFooter
          variant='sign-in'
          status={{ user_agreement_enabled: true }}
        />
      </>
    )

    expect(view.container).toHaveTextContent('我已阅读并同意服务协议。')
    expect(view.container).toHaveTextContent(
      '点击登录，即表示您同意我们的服务协议。'
    )
    expect(view.getAllByRole('link', { name: '服务协议' })).toHaveLength(2)
    expect(
      view.queryByRole('link', { name: '隐私政策' })
    ).not.toBeInTheDocument()
  })

  test('mentions only the privacy policy when the terms are disabled', async () => {
    const view = await renderInChinese(
      <>
        <LegalConsent
          status={{ privacy_policy_enabled: true }}
          checked={false}
          onCheckedChange={() => undefined}
        />
        <TermsFooter
          variant='sign-in'
          status={{ privacy_policy_enabled: true }}
        />
      </>
    )

    expect(view.container).toHaveTextContent('我已阅读并同意隐私政策。')
    expect(view.container).toHaveTextContent(
      '点击登录，即表示您同意我们的隐私政策。'
    )
    expect(view.getAllByRole('link', { name: '隐私政策' })).toHaveLength(2)
    expect(
      view.queryByRole('link', { name: '服务协议' })
    ).not.toBeInTheDocument()
  })
})
