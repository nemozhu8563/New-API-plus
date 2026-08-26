import { zodResolver } from '@hookform/resolvers/zod'
import { type FormEvent, useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DateTimePicker } from '@/components/datetime-picker'
import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { getAdminPlans } from '@/features/subscriptions/api'
import type { PlanRecord } from '@/features/subscriptions/types'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import { formatQuota, parseQuotaFromDollars } from '@/lib/format'
import { addTimeToDate } from '@/lib/time'

import { createRedemption, updateRedemption, getRedemption } from '../api'
import { SUCCESS_MESSAGES } from '../constants'
import {
  getRedemptionFormSchema,
  type RedemptionFormValues,
  REDEMPTION_FORM_DEFAULT_VALUES,
  truncateRedemptionName,
  transformFormDataToPayload,
  transformRedemptionToFormDefaults,
} from '../lib'
import type { Redemption } from '../types'
import { useRedemptions } from './redemptions-provider'

type RedemptionsMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: Redemption
}

export function RedemptionsMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: RedemptionsMutateDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = !!currentRow
  const { triggerRefresh } = useRedemptions()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [loadedRedemptionId, setLoadedRedemptionId] = useState<number | null>(
    null
  )
  const [plans, setPlans] = useState<PlanRecord[]>([])
  const [plansLoading, setPlansLoading] = useState(false)

  const form = useForm<RedemptionFormValues>({
    resolver: zodResolver(getRedemptionFormSchema(t)),
    defaultValues: REDEMPTION_FORM_DEFAULT_VALUES,
  })

  // Load existing data when updating
  useEffect(() => {
    let cancelled = false

    if (open && isUpdate && currentRow) {
      // For update, fetch fresh data
      setLoadedRedemptionId(null)
      getRedemption(currentRow.id)
        .then((result) => {
          if (cancelled) return
          if (!result.success || !result.data) {
            toast.error(t('Loading failed'))
            return
          }
          form.reset(transformRedemptionToFormDefaults(result.data))
          setLoadedRedemptionId(currentRow.id)
        })
        .catch(() => {
          if (!cancelled) {
            toast.error(t('Loading failed'))
          }
        })
    } else {
      setLoadedRedemptionId(null)
      if (open && !isUpdate) {
        // For create, reset to defaults
        form.reset(REDEMPTION_FORM_DEFAULT_VALUES)
      }
    }

    return () => {
      cancelled = true
    }
  }, [open, isUpdate, currentRow, form, t])

  const isLoadingRedemption =
    open && isUpdate && loadedRedemptionId !== currentRow?.id
  let submitLabel = t('Save changes')
  if (isLoadingRedemption) {
    submitLabel = t('Loading...')
  } else if (isSubmitting) {
    submitLabel = t('Saving...')
  }

  useEffect(() => {
    if (!open || isUpdate) return

    let cancelled = false
    setPlansLoading(true)
    getAdminPlans()
      .then((result) => {
        if (cancelled) return
        setPlans(
          result.success
            ? (result.data || []).filter((record) => record.plan.enabled)
            : []
        )
      })
      .finally(() => {
        if (!cancelled) setPlansLoading(false)
      })
      .catch(() => {
        if (!cancelled) {
          setPlans([])
          toast.error(t('Loading failed'))
        }
      })

    return () => {
      cancelled = true
    }
  }, [open, isUpdate, t])

  const onSubmit = async (data: RedemptionFormValues) => {
    setIsSubmitting(true)
    try {
      const basePayload = transformFormDataToPayload(data)

      if (isUpdate && currentRow) {
        const result = await updateRedemption({
          ...basePayload,
          id: currentRow.id,
        })
        if (result.success) {
          toast.success(t(SUCCESS_MESSAGES.REDEMPTION_UPDATED))
          onOpenChange(false)
          triggerRefresh()
        }
      } else {
        // Create mode
        const result = await createRedemption(basePayload)
        if (result.success) {
          const count = result.data?.length || 0
          toast.success(
            count > 1
              ? t('Successfully created {{count}} redemption codes', {
                  count,
                })
              : t(SUCCESS_MESSAGES.REDEMPTION_CREATED)
          )
          onOpenChange(false)
          triggerRefresh()
        }
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    if (isLoadingRedemption) {
      event.preventDefault()
      return
    }

    if (!isUpdate) {
      const name = form.getValues('name')
      if (!name?.trim()) {
        if (form.getValues('benefit_type') === 'subscription') {
          const planId = Number(form.getValues('subscription_plan_id'))
          const plan = plans.find((record) => record.plan.id === planId)
          form.setValue(
            'name',
            truncateRedemptionName(plan?.plan.title || t('Subscription')),
            { shouldValidate: true }
          )
        } else {
          const quota = parseQuotaFromDollars(form.getValues('quota_dollars'))
          form.setValue('name', formatQuota(quota), { shouldValidate: true })
        }
      }
    }

    void form.handleSubmit(onSubmit)(event)
  }

  const handleSetExpiry = (months: number, days: number, hours: number) => {
    const newDate = addTimeToDate(months, days, hours)
    form.setValue('expired_time', newDate)
  }

  const { meta: currencyMeta } = getCurrencyDisplay()
  const currencyLabel = getCurrencyLabel()
  const tokensOnly = currencyMeta.kind === 'tokens'
  const quotaLabel = t('Quota ({{currency}})', { currency: currencyLabel })
  const quotaPlaceholder = tokensOnly
    ? t('Enter quota in tokens')
    : t('Enter quota in {{currency}}', { currency: currencyLabel })
  const benefitType = form.watch('benefit_type')

  return (
    <Sheet
      open={open}
      onOpenChange={(v) => {
        onOpenChange(v)
        if (!v) {
          form.reset()
        }
      }}
    >
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[600px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isUpdate
              ? t('Update Redemption Code')
              : t('Create Redemption Code')}
          </SheetTitle>
          <SheetDescription>
            {isUpdate
              ? t('Update the redemption code by providing necessary info.')
              : t(
                  'Add new redemption code(s) by providing necessary info.'
                )}{' '}
            {t('Click save when you&apos;re done.')}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='redemption-form'
            onSubmit={handleSubmit}
            className={sideDrawerFormClassName()}
          >
            <SideDrawerSection>
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder={t('Enter a name')} />
                    </FormControl>
                    <FormDescription>
                      {t('Name for this redemption code (1-20 characters)')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='benefit_type'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Type')}</FormLabel>
                    <Select
                      items={[
                        { value: 'quota', label: t('Quota') },
                        { value: 'subscription', label: t('Subscription') },
                      ]}
                      value={field.value}
                      onValueChange={(value) =>
                        value !== null && field.onChange(value)
                      }
                    >
                      <FormControl>
                        <SelectTrigger disabled={isUpdate}>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          <SelectItem value='quota'>{t('Quota')}</SelectItem>
                          <SelectItem value='subscription'>
                            {t('Subscription')}
                          </SelectItem>
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      {t('The benefit type cannot be changed after creation')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {benefitType === 'quota' ? (
                <FormField
                  control={form.control}
                  name='quota_dollars'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{quotaLabel}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          step={tokensOnly ? 1 : 0.01}
                          placeholder={quotaPlaceholder}
                          onChange={(e) =>
                            field.onChange(
                              Number.parseFloat(e.target.value) || 0
                            )
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {tokensOnly
                          ? t('Enter the quota amount in tokens')
                          : t('Enter the quota amount in {{currency}}', {
                              currency: currencyLabel,
                            })}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              ) : (
                <FormField
                  control={form.control}
                  name='subscription_plan_id'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Subscription')}</FormLabel>
                      {isUpdate ? (
                        <FormControl>
                          <Input
                            value={
                              currentRow?.subscription_plan_title ||
                              t('Plan #{{id}}', {
                                id: currentRow?.subscription_plan_id || '-',
                              })
                            }
                            disabled
                          />
                        </FormControl>
                      ) : (
                        <Select
                          items={plans.map((record) => ({
                            value: String(record.plan.id),
                            label: record.plan.title,
                          }))}
                          value={field.value}
                          onValueChange={(value) =>
                            value !== null && field.onChange(value)
                          }
                        >
                          <FormControl>
                            <SelectTrigger disabled={plansLoading}>
                              <SelectValue
                                placeholder={
                                  plansLoading
                                    ? t('Loading...')
                                    : t('Select subscription plan')
                                }
                              />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              {plans.map((record) => (
                                <SelectItem
                                  key={record.plan.id}
                                  value={String(record.plan.id)}
                                >
                                  {record.plan.title}
                                </SelectItem>
                              ))}
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                      )}
                      <FormDescription>
                        {t(
                          'Subscription benefits are frozen when codes are created'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}

              <FormField
                control={form.control}
                name='expired_time'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Expiration Time')}</FormLabel>
                    <div className='flex flex-col gap-2'>
                      <FormControl>
                        <DateTimePicker
                          value={field.value}
                          onChange={field.onChange}
                          placeholder={t('Never expires')}
                        />
                      </FormControl>
                      <div className='grid grid-cols-4 gap-1.5 sm:flex sm:gap-2'>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          onClick={() => handleSetExpiry(0, 0, 0)}
                        >
                          {t('Never')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          onClick={() => handleSetExpiry(1, 0, 0)}
                        >
                          {t('1M')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          onClick={() => handleSetExpiry(0, 7, 0)}
                        >
                          {t('1W')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          onClick={() => handleSetExpiry(0, 1, 0)}
                        >
                          {t('1 Day')}
                        </Button>
                      </div>
                    </div>
                    <FormDescription>
                      {t('Leave empty for never expires')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {!isUpdate && (
                <FormField
                  control={form.control}
                  name='count'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Quantity')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          min='1'
                          max='100'
                          placeholder={t('Number of codes to create')}
                          onChange={(e) =>
                            field.onChange(
                              Number.parseInt(e.target.value, 10) || 1
                            )
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Create multiple redemption codes at once (1-100)')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}
            </SideDrawerSection>
          </form>
        </Form>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button variant='outline' />}>
            {t('Close')}
          </SheetClose>
          <Button
            form='redemption-form'
            type='submit'
            disabled={isSubmitting || isLoadingRedemption}
          >
            {submitLabel}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
