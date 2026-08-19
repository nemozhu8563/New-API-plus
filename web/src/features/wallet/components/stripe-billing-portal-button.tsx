import { ExternalLink, Loader2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { createStripeBillingPortalSession } from '@/features/subscriptions/api'

import { redirectToHostedCheckout } from '../lib/payment'

export function StripeBillingPortalButton() {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(false)

  const handleClick = async () => {
    setLoading(true)
    try {
      const response = await createStripeBillingPortalSession()
      const portalUrl = response.data?.portal_url
      if (!response.success || !portalUrl) {
        toast.error(t('Unable to open billing portal'))
        return
      }
      redirectToHostedCheckout(portalUrl)
    } catch {
      toast.error(t('Unable to open billing portal'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Button
      variant='outline'
      size='sm'
      className='h-8'
      onClick={handleClick}
      disabled={loading}
    >
      {loading ? (
        <Loader2 className='h-3.5 w-3.5 animate-spin' aria-hidden='true' />
      ) : (
        <ExternalLink className='h-3.5 w-3.5' aria-hidden='true' />
      )}
      {loading ? t('Opening billing portal...') : t('Manage billing')}
    </Button>
  )
}
