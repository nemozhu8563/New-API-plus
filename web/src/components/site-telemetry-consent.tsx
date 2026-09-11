import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  getAnalyticsConsent,
  isSiteTelemetryEnabled,
  setAnalyticsConsent,
} from '@/lib/site-telemetry'

export function SiteTelemetryConsent() {
  const { t } = useTranslation()
  const [visible, setVisible] = useState(
    () => isSiteTelemetryEnabled() && getAnalyticsConsent() === null
  )

  if (!visible) return null

  const choose = (consent: 'granted' | 'denied') => {
    setAnalyticsConsent(consent)
    setVisible(false)
  }

  return (
    <aside
      className='bg-background fixed right-4 bottom-4 left-4 z-50 mx-auto max-w-xl rounded-xl border p-4 shadow-lg sm:left-auto'
      role='dialog'
      aria-labelledby='analytics-consent-title'
    >
      <p id='analytics-consent-title' className='text-sm font-medium'>
        {t('Help us improve Tryvalo with analytics cookies.')}
      </p>
      <p className='text-muted-foreground mt-1 text-sm'>
        {t('You can learn more in our Privacy Policy.')}{' '}
        <a
          className='text-primary underline underline-offset-4'
          href='/privacy-policy'
        >
          {t('Privacy Policy')}
        </a>
      </p>
      <div className='mt-3 flex flex-wrap justify-end gap-2'>
        <Button variant='outline' onClick={() => choose('denied')}>
          {t('Reject analytics')}
        </Button>
        <Button onClick={() => choose('granted')}>
          {t('Allow analytics')}
        </Button>
      </div>
    </aside>
  )
}
