import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { handleServerError } from '@/lib/handle-server-error'
import { AuthOperationError } from '@/lib/secure-verification'
import { useAuthStore } from '@/stores/auth-store'

import { useAuthRedirect } from '../../hooks/use-auth-redirect'
import {
  SecureVerificationDialog,
  useSecureVerification,
} from '../../secure-verification'

export function OtpForm() {
  const { t } = useTranslation()
  // Transfer the pending challenge from the navigation handoff into this page's
  // lifetime. Reloading or leaving the page cannot resume it from browser storage.
  const pending = useRef(
    useAuthStore.getState().auth.pendingLoginVerification
  ).current
  const initialSessionID = useRef(
    useAuthStore.getState().auth.session?.sid
  ).current
  const sessionID = useAuthStore((state) => state.auth.session?.sid)
  const verification = useSecureVerification()
  const { requestLoginVerification, cancel } = verification
  const { handleLoginSuccess, redirectToLogin } = useAuthRedirect()
  const completed = useRef(false)

  useEffect(() => {
    if (completed.current) return
    if (!pending) {
      redirectToLogin()
      return
    }
    if (sessionID !== initialSessionID) {
      cancel()
      return
    }
    if (useAuthStore.getState().auth.pendingLoginVerification === pending) {
      useAuthStore.getState().auth.setPendingLoginVerification(null)
    }
    let active = true
    void (async () => {
      try {
        const bundle = await requestLoginVerification(pending.challenge)
        if (
          !active ||
          useAuthStore.getState().auth.session?.sid !== initialSessionID
        ) {
          return
        }
        if (!bundle) {
          redirectToLogin()
          return
        }
        completed.current = true
        await handleLoginSuccess(bundle, pending.redirectTo)
        toast.success(t('Signed in'))
      } catch (error) {
        if (!active) return
        handleServerError(AuthOperationError.from(error))
        redirectToLogin()
      }
      const res = await login2fa({
        code,
        flow_token: pending2FAFlowToken,
      })

      if (!res.success) {
        if (getServerErrorMessageKey(res)) return
        toast.error(res.message || t('Invalid code'))
        return
      }

      if (!res.data) {
        throw new Error(t('Login failed'))
      }

      await handleLoginSuccess(res.data, undefined, { method: 'otp' })
      toast.success(t('Signed in'))
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('2FA verification error:', error)
      if (getServerErrorMessageKey(error)) return
      const errorMessage =
        error instanceof Error ? error.message : t('Verification failed')
      toast.error(errorMessage)
    } finally {
      setIsLoading(false)
    }
  }, [
    pending,
    initialSessionID,
    sessionID,
    requestLoginVerification,
    cancel,
    handleLoginSuccess,
    redirectToLogin,
    t,
  ])

  return <SecureVerificationDialog {...verification.dialogProps} />
}
