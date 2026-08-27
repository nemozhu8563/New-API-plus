import { AlertTriangle, CalendarClock, FileText, Loader2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { cancelStripeSubscription } from '@/features/subscriptions/api'
import type {
  StripeInvoiceSummary,
  StripeSubscriptionSummary,
} from '@/features/subscriptions/types'
import { toIntlLocale } from '@/i18n/languages'
import { formatQuota } from '@/lib/format'

import { StripeBillingPortalButton } from './stripe-billing-portal-button'
import { formatStripeInvoiceAmount } from './stripe-invoice-amount'

interface StripeSubscriptionBillingProps {
  subscriptions: StripeSubscriptionSummary[]
  invoices: StripeInvoiceSummary[]
  billingDebt: number
  onRefresh: () => Promise<void>
}

function subscriptionStatusVariant(status: string) {
  if (status === 'active' || status === 'trialing') return 'success' as const
  if (status === 'past_due' || status === 'unpaid') return 'warning' as const
  return 'neutral' as const
}

export function StripeSubscriptionBilling(
  props: StripeSubscriptionBillingProps
) {
  const { t, i18n } = useTranslation()
  const intlLocale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const [cancelTarget, setCancelTarget] =
    useState<StripeSubscriptionSummary | null>(null)
  const [isCancelling, setIsCancelling] = useState(false)

  if (
    props.subscriptions.length === 0 &&
    props.invoices.length === 0 &&
    props.billingDebt <= 0
  ) {
    return null
  }

  const formatDateTime = (timestamp: number) => {
    if (timestamp <= 0) return t('Not available')
    return new Intl.DateTimeFormat(intlLocale, {
      dateStyle: 'medium',
      timeStyle: 'short',
    }).format(new Date(timestamp * 1000))
  }

  const handleCancel = async () => {
    if (!cancelTarget) return
    setIsCancelling(true)
    try {
      const response = await cancelStripeSubscription(
        cancelTarget.subscription_id
      )
      if (!response.success) {
        toast.error(response.message || t('Unable to cancel subscription'))
        return
      }
      toast.success(t('Subscription will end after the current billing period'))
      setCancelTarget(null)
      await props.onRefresh()
    } catch {
      toast.error(t('Unable to cancel subscription'))
    } finally {
      setIsCancelling(false)
    }
  }

  return (
    <section className='rounded-xl border p-3 sm:p-4'>
      <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
        <div>
          <h3 className='text-sm font-medium'>{t('Stripe billing')}</h3>
          <p className='text-muted-foreground mt-1 text-xs leading-5 text-pretty'>
            {t(
              'Review renewal and cancellation status here. Use Stripe only to update payment methods or open complete invoices.'
            )}
          </p>
        </div>
        {(props.subscriptions.length > 0 || props.invoices.length > 0) && (
          <StripeBillingPortalButton />
        )}
      </div>

      {props.billingDebt > 0 && (
        <div
          role='alert'
          className='border-destructive/35 bg-destructive/5 mt-4 flex gap-3 rounded-lg border p-3'
        >
          <AlertTriangle
            className='text-destructive mt-0.5 size-4 shrink-0'
            aria-hidden='true'
          />
          <div>
            <p className='text-sm font-medium'>{t('Outstanding balance')}</p>
            <p className='text-muted-foreground mt-1 text-xs leading-5 text-pretty'>
              {t(
                'A refunded or disputed payment left {{amount}} outstanding. New top-ups pay this balance first, and API requests remain paused until it is cleared.',
                { amount: formatQuota(props.billingDebt) }
              )}
            </p>
          </div>
        </div>
      )}

      {props.subscriptions.length > 0 && (
        <div className='mt-4 space-y-3'>
          {props.subscriptions.map((subscription) => {
            const canCancel =
              (subscription.status === 'active' ||
                subscription.status === 'trialing') &&
              !subscription.cancel_at_period_end
            return (
              <article
                key={subscription.subscription_id}
                className='bg-background rounded-lg border p-3'
              >
                <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
                  <div className='min-w-0'>
                    <div className='flex flex-wrap items-center gap-2'>
                      <span className='text-sm font-medium'>
                        {subscription.plan_title
                          ? t(subscription.plan_title)
                          : t('Stripe subscription')}
                      </span>
                      <StatusBadge
                        label={
                          subscription.cancel_at_period_end
                            ? t('Cancellation scheduled')
                            : t(subscription.status || 'Unknown')
                        }
                        variant={
                          subscription.cancel_at_period_end
                            ? 'warning'
                            : subscriptionStatusVariant(subscription.status)
                        }
                        copyable={false}
                      />
                    </div>
                    <div className='text-muted-foreground mt-2 flex items-start gap-2 text-xs leading-5'>
                      <CalendarClock
                        className='mt-0.5 size-3.5 shrink-0'
                        aria-hidden='true'
                      />
                      <span>
                        {subscription.cancel_at_period_end
                          ? t('Access ends on {{date}}', {
                              date: formatDateTime(
                                subscription.current_period_end
                              ),
                            })
                          : t('Next billing date: {{date}}', {
                              date: formatDateTime(
                                subscription.current_period_end
                              ),
                            })}
                      </span>
                    </div>
                  </div>
                  {canCancel && (
                    <Button
                      variant='outline'
                      size='sm'
                      className='h-8 shrink-0'
                      onClick={() => setCancelTarget(subscription)}
                    >
                      {t('Cancel subscription')}
                    </Button>
                  )}
                </div>
              </article>
            )
          })}
        </div>
      )}

      <Separator className='my-4' />

      <div>
        <div className='flex items-center gap-2'>
          <FileText className='size-4' aria-hidden='true' />
          <h3 className='text-sm font-medium'>{t('Billing history')}</h3>
        </div>
        <p className='text-muted-foreground mt-1 text-xs leading-5 text-pretty'>
          {t(
            'Showing the 8 most recent invoices. Open Stripe for complete history.'
          )}
        </p>
        {props.invoices.length === 0 ? (
          <p className='text-muted-foreground mt-2 text-xs'>
            {t('No Stripe invoices yet')}
          </p>
        ) : (
          <div className='mt-2 divide-y' role='list'>
            {props.invoices.slice(0, 8).map((invoice) => (
              <div
                key={invoice.invoice_id}
                className='flex flex-col gap-1 py-2.5 text-xs sm:flex-row sm:items-center sm:justify-between'
                role='listitem'
              >
                <div className='min-w-0'>
                  <p className='truncate font-medium'>
                    {invoice.plan_title
                      ? t(invoice.plan_title)
                      : t('Stripe invoice')}
                  </p>
                  <p className='text-muted-foreground mt-0.5'>
                    {t('{{start}} to {{end}}', {
                      start: formatDateTime(invoice.period_start),
                      end: formatDateTime(invoice.period_end),
                    })}
                  </p>
                </div>
                <span className='font-medium tabular-nums'>
                  {formatStripeInvoiceAmount(
                    invoice.amount_paid_minor,
                    invoice.currency,
                    intlLocale
                  )}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>

      <ConfirmDialog
        open={cancelTarget !== null}
        onOpenChange={(open) => !open && setCancelTarget(null)}
        title={t('Cancel Stripe subscription?')}
        desc={t(
          'Automatic renewal will stop. Your paid subscription and its quota remain available until {{date}}.',
          {
            date: cancelTarget
              ? formatDateTime(cancelTarget.current_period_end)
              : '',
          }
        )}
        confirmText={
          isCancelling ? (
            <>
              <Loader2 className='size-4 animate-spin' aria-hidden='true' />
              {t('Cancelling...')}
            </>
          ) : (
            t('Confirm cancellation')
          )
        }
        handleConfirm={handleCancel}
        isLoading={isCancelling}
        destructive
      />
    </section>
  )
}
