import { Trans, useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import type { SystemStatus } from '../types'

interface TermsFooterProps {
  variant?: 'sign-in' | 'sign-up'
  className?: string
  status?: SystemStatus | null
}

export function TermsFooter({
  variant = 'sign-in',
  className,
  status,
}: TermsFooterProps) {
  const { t } = useTranslation()
  const hasUserAgreement = Boolean(status?.user_agreement_enabled)
  const hasPrivacyPolicy = Boolean(status?.privacy_policy_enabled)

  if (!hasUserAgreement && !hasPrivacyPolicy) {
    return null
  }

  let textKey: string
  if (variant === 'sign-in') {
    if (hasUserAgreement && hasPrivacyPolicy) {
      textKey =
        'By clicking sign in, you agree to our Terms of Service and Privacy Policy.'
    } else if (hasUserAgreement) {
      textKey = 'By clicking sign in, you agree to our Terms of Service.'
    } else {
      textKey = 'By clicking sign in, you agree to our Privacy Policy.'
    }
  } else if (hasUserAgreement && hasPrivacyPolicy) {
    textKey =
      'By creating an account, you agree to our Terms of Service and Privacy Policy.'
  } else if (hasUserAgreement) {
    textKey = 'By creating an account, you agree to our Terms of Service.'
  } else {
    textKey = 'By creating an account, you agree to our Privacy Policy.'
  }

  return (
    <p className={cn('text-muted-foreground text-center text-xs', className)}>
      <Trans
        t={t}
        i18nKey={textKey}
        components={{
          terms: (
            <a
              href='/user-agreement'
              className='hover:text-primary underline underline-offset-4'
            />
          ),
          privacy: (
            <a
              href='/privacy-policy'
              className='hover:text-primary underline underline-offset-4'
            />
          ),
        }}
      />
    </p>
  )
}
