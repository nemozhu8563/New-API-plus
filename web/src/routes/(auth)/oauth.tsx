import { createFileRoute, useNavigate, useSearch } from '@tanstack/react-router'
import i18next from 'i18next'
import { useEffect } from 'react'

import { wechatLoginByCode } from '@/features/auth/api'
import { sanitizeAuthRedirect } from '@/features/auth/lib/auth-redirect'
import { applyAuthBundle, isAuthBundle } from '@/lib/api'
import { DEFAULT_CONSOLE_ROUTE } from '@/lib/app-entry-route'
import { getServerErrorMessageKey } from '@/lib/server-error-message'
import { trackEvent } from '@/lib/site-telemetry'

function OAuthComponent() {
  const navigate = useNavigate()
  const search = useSearch({ from: '/(auth)/oauth' }) as {
    redirect?: string
    provider?: 'github' | 'discord' | 'oidc' | 'linuxdo' | 'telegram' | 'wechat'
    code?: string
    state?: string
  }

  useEffect(() => {
    ;(async () => {
      try {
        if (search?.provider === 'wechat' && search.code) {
          const res = await wechatLoginByCode(search.code)
          if (res?.success && isAuthBundle(res.data)) {
            applyAuthBundle(res.data)
            trackEvent('login', { method: 'wechat' })
            const target =
              sanitizeAuthRedirect(search?.redirect, window.location.origin) ??
              DEFAULT_CONSOLE_ROUTE
            navigate({ href: target, replace: true })
            return
          }
          throw createServerError(res, i18next.t('OAuth failed'))
        }
        handleServerError(new AuthOperationError(i18next.t('OAuth failed')))
      } catch (error: unknown) {
        handleServerError(
          AuthOperationError.from(error, i18next.t('OAuth failed'))
        )
      }
      navigate({ to: '/sign-in', replace: true })
    })()
  }, [navigate, search])

  return null
}

export const Route = createFileRoute('/(auth)/oauth')({
  component: OAuthComponent,
})
