import { Loader2, Minus, Plus, ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  DEFAULT_STRIPE_MAX_TOPUP,
  DEFAULT_STRIPE_TOPUP_UNIT,
  STRIPE_TOPUP_PRESET_MULTIPLIERS,
} from '../constants'

interface StripeTopupSectionProps {
  topupAmount: number
  unit?: number
  maxAmount?: number
  processing: boolean
  onAmountChange: (amount: number) => void
  onCheckout: () => void | Promise<void>
}

function formatCredits(amount: number): string {
  return `$${formatNumber(amount)} USD`
}

function formatCny(amount: number): string {
  return `¥${formatNumber(amount)}`
}

export function StripeTopupSection(props: StripeTopupSectionProps) {
  const { t } = useTranslation()
  const unit =
    props.unit && props.unit > 0 ? props.unit : DEFAULT_STRIPE_TOPUP_UNIT
  const maxAmount =
    props.maxAmount && props.maxAmount >= unit
      ? props.maxAmount
      : DEFAULT_STRIPE_MAX_TOPUP
  const hasValidAmount =
    props.topupAmount >= unit &&
    props.topupAmount <= maxAmount &&
    props.topupAmount % unit === 0
  const selectedAmount = hasValidAmount ? props.topupAmount : unit
  const quantity = selectedAmount / unit
  const presetAmounts = STRIPE_TOPUP_PRESET_MULTIPLIERS.map(
    (multiplier) => multiplier * unit
  ).filter((amount) => amount <= maxAmount)

  const changeQuantity = (nextQuantity: number) => {
    const nextAmount = nextQuantity * unit
    if (nextAmount < unit || nextAmount > maxAmount) return
    props.onAmountChange(nextAmount)
  }

  return (
    <div className='grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(220px,0.68fr)]'>
      <div className='space-y-4'>
        <div className='space-y-2.5 sm:space-y-3'>
          <Label className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
            {t('Amount')}
          </Label>
          <div className='grid grid-cols-2 gap-2 sm:grid-cols-3'>
            {presetAmounts.map((amount) => {
              const selected = amount === selectedAmount
              return (
                <Button
                  key={amount}
                  type='button'
                  variant='outline'
                  aria-pressed={selected}
                  className={cn(
                    'flex min-h-20 flex-col items-center justify-center rounded-xl px-3 py-3 text-center',
                    selected
                      ? 'border-primary bg-primary/5 text-primary ring-primary/20 ring-2'
                      : 'border-muted'
                  )}
                  onClick={() => props.onAmountChange(amount)}
                >
                  <span className='text-xl font-semibold'>
                    {formatCny(amount)}
                  </span>
                  <span
                    className={cn(
                      'mt-1 text-xs font-normal',
                      selected ? 'text-primary' : 'text-muted-foreground'
                    )}
                  >
                    {formatCredits(amount)}
                  </span>
                </Button>
              )
            })}
          </div>
        </div>

        <div className='flex flex-col gap-3 rounded-xl border p-3 sm:flex-row sm:items-center sm:justify-between sm:p-4'>
          <div>
            <p className='text-sm font-medium'>{t('Purchase quantity')}</p>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t('Each package includes {{credits}} ({{price}})', {
                credits: formatCredits(unit),
                price: formatCny(unit),
              })}
            </p>
          </div>
          <div className='grid grid-cols-[2.5rem_3.5rem_2.5rem] overflow-hidden rounded-lg border'>
            <Button
              type='button'
              variant='ghost'
              size='icon'
              className='rounded-none border-r'
              aria-label={t('Decrease quantity')}
              disabled={quantity <= 1 || props.processing}
              onClick={() => changeQuantity(quantity - 1)}
            >
              <Minus className='h-4 w-4' />
            </Button>
            <output
              className='flex items-center justify-center text-sm font-semibold'
              aria-label={t('Quantity')}
            >
              {quantity}
            </output>
            <Button
              type='button'
              variant='ghost'
              size='icon'
              className='rounded-none border-l'
              aria-label={t('Increase quantity')}
              disabled={selectedAmount >= maxAmount || props.processing}
              onClick={() => changeQuantity(quantity + 1)}
            >
              <Plus className='h-4 w-4' />
            </Button>
          </div>
        </div>
      </div>

      <aside
        className='bg-muted/25 flex flex-col justify-between rounded-xl border p-4'
        aria-label={t('Order summary')}
      >
        <div className='space-y-3 text-sm' aria-live='polite'>
          <h3 className='font-semibold'>{t('Order summary')}</h3>
          <div className='flex items-center justify-between gap-3'>
            <span className='text-muted-foreground'>{t('Credits added')}</span>
            <span className='font-medium'>{formatCredits(selectedAmount)}</span>
          </div>
          <div className='flex items-center justify-between gap-3'>
            <span className='text-muted-foreground'>{t('Quantity')}</span>
            <span className='font-medium'>{quantity}</span>
          </div>
          <div className='border-t pt-3'>
            <div className='flex items-end justify-between gap-3'>
              <span className='text-muted-foreground'>{t('Total')}</span>
              <span className='text-primary text-3xl font-semibold'>
                {formatCny(selectedAmount)}
              </span>
            </div>
          </div>
        </div>

        <Button
          type='button'
          className='mt-5 w-full gap-2'
          disabled={!hasValidAmount || props.processing}
          aria-busy={props.processing}
          onClick={() => void props.onCheckout()}
        >
          {props.processing ? (
            <Loader2 className='h-4 w-4 animate-spin' />
          ) : (
            <ShieldCheck className='h-4 w-4' />
          )}
          {t('Pay')}
        </Button>
      </aside>
    </div>
  )
}
