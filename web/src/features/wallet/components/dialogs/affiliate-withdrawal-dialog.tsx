import { Loader2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  formatQuota,
  parseQuotaFromDollars,
  quotaUnitsToDollars,
} from '@/lib/format'

interface AffiliateWithdrawalDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: (amountQuota: number, note: string) => Promise<boolean>
  availableQuota: number
  submitting: boolean
}

export function AffiliateWithdrawalDialog({
  open,
  onOpenChange,
  onConfirm,
  availableQuota,
  submitting,
}: AffiliateWithdrawalDialogProps) {
  const { t } = useTranslation()
  const [amount, setAmount] = useState(0)
  const [note, setNote] = useState('')
  const amountQuota = parseQuotaFromDollars(amount)
  const maximumAmount = quotaUnitsToDollars(availableQuota)
  const canSubmit =
    Number.isFinite(amount) &&
    amountQuota > 0 &&
    amountQuota <= availableQuota &&
    note.length <= 500

  useEffect(() => {
    if (open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setAmount(0)
      setNote('')
    }
  }, [open])

  const handleConfirm = async () => {
    if (!canSubmit) return
    const success = await onConfirm(amountQuota, note)
    if (success) {
      onOpenChange(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Request cashback withdrawal')}
      description={t(
        'The amount is frozen after submission. An administrator confirms the offline payment or rejects and returns it.'
      )}
      contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'
      contentHeight='auto'
      bodyClassName='space-y-4 py-2'
      footer={
        <>
          <Button
            variant='outline'
            onClick={() => onOpenChange(false)}
            disabled={submitting}
          >
            {t('Cancel')}
          </Button>
          <Button onClick={handleConfirm} disabled={!canSubmit || submitting}>
            {submitting ? <Loader2 className='animate-spin' /> : null}
            {t('Submit request')}
          </Button>
        </>
      }
    >
      <div className='space-y-2'>
        <Label>{t('Available cashback')}</Label>
        <div className='text-xl font-semibold tabular-nums'>
          {formatQuota(availableQuota)}
        </div>
      </div>
      <div className='space-y-2'>
        <Label htmlFor='affiliate-withdrawal-amount'>
          {t('Withdrawal amount')}
        </Label>
        <Input
          id='affiliate-withdrawal-amount'
          type='number'
          min={0}
          max={maximumAmount}
          step='any'
          value={amount}
          onChange={(event) => setAmount(Number(event.target.value))}
        />
      </div>
      <div className='space-y-2'>
        <Label htmlFor='affiliate-withdrawal-note'>
          {t('Withdrawal note')}
        </Label>
        <Textarea
          id='affiliate-withdrawal-note'
          value={note}
          maxLength={500}
          onChange={(event) => setNote(event.target.value)}
          placeholder={t('Add offline payment details or a note for review')}
        />
        <p className='text-muted-foreground text-right text-xs'>
          {note.length}/500
        </p>
      </div>
    </Dialog>
  )
}
