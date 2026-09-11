import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { LobeIconField } from '@/components/lobe-icon-field'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { formatTimestampToDate } from '@/lib/format'
import { createServerError } from '@/lib/server-error-message'

import { createVendor, getVendor, updateVendor } from '../../api'
import { vendorsQueryKeys } from '../../lib'
import {
  vendorFormSchema,
  type Vendor,
  type VendorFormValues,
} from '../../types'
import { invalidateVendorData, vendorErrorMessage } from '../../vendor-api'
import { VendorLinkedModels } from '../vendor-linked-models'

export function VendorMutateDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentVendor?: Vendor | null
}) {
  const { t } = useTranslation()
  const client = useQueryClient()
  const id = props.currentVendor?.id
  const [section, setSection] = useState('details')
  const [confirmClose, setConfirmClose] = useState(false)
  const loadedKey = useRef('')
  const pendingNavigation = useRef<(() => void) | null>(null)
  const form = useForm({
    resolver: zodResolver(vendorFormSchema),
    defaultValues: { name: '', description: '', icon: '', version: '' },
  })
  const query = useQuery({
    queryKey: vendorsQueryKeys.detail(id ?? 0),
    queryFn: async () => {
      if (!id) throw new Error(t('Select a saved vendor.'))
      const response = await getVendor(id)
      if (!response.success || !response.data) {
        throw createServerError(response, t('Failed to load vendor'))
      }
      return response.data
    },
    enabled: props.open && Boolean(id),
  })
  useEffect(() => {
    if (!props.open) {
      loadedKey.current = ''
      return
    }
    const key = String(id ?? 'new')
    if (loadedKey.current === key || (id && !query.data)) return
    const value = query.data
    form.reset({
      name: id ? (value?.name ?? '') : '',
      description: id ? (value?.description ?? '') : '',
      icon: id ? (value?.icon ?? '') : '',
      version: id ? (value?.version ?? '') : '',
    })
    loadedKey.current = key
    setSection('details')
  }, [props.open, id, query.data, form])
  const save = useMutation({
    mutationFn: async (values: VendorFormValues) => {
      const response = id
        ? await updateVendor({ ...values, id })
        : await createVendor(values)
      if (!response.success) {
        throw createServerError(response, t('Operation failed'))
      }
      return response.data
    },
    onSuccess: async () => {
      await invalidateVendorData(client)
      toast.success(
        id ? t('Vendor updated successfully') : t('Vendor created successfully')
      )
      form.reset(form.getValues())
      props.onOpenChange(false)
    },
  })

  // Load vendor data for editing
  useEffect(() => {
    if (open && isEdit && currentVendor) {
      form.reset({
        id: currentVendor.id,
        name: currentVendor.name,
        description: currentVendor.description || '',
        icon: currentVendor.icon || '',
        status: currentVendor.status || 1,
      })
    } else if (open && !isEdit) {
      form.reset({
        name: '',
        description: '',
        icon: '',
        status: 1,
      })
    }
  }, [open, isEdit, currentVendor, form])

  const onSubmit = async (values: Record<string, unknown>) => {
    setIsSaving(true)
    try {
      let response
      if (isEdit) {
        if (!currentVendor) {
          toast.error(t('Vendor not found'))
          return
        }
        response = await updateVendor({ ...values, id: currentVendor.id })
      } else {
        response = await createVendor(values)
      }

      if (response.success) {
        toast.success(
          isEdit ? 'Vendor updated successfully' : 'Vendor created successfully'
        )
        queryClient.invalidateQueries({ queryKey: vendorsQueryKeys.lists() })
        queryClient.invalidateQueries({ queryKey: modelsQueryKeys.lists() })
        onOpenChange(false)
      } else {
        toast.error(response.message || 'Operation failed')
      }
    } catch (error: unknown) {
      toast.error((error as Error)?.message || 'Operation failed')
    } finally {
      setIsSaving(false)
    }
    props.onOpenChange(false)
    return true
  }

  let submitLabel = t('Create')
  if (isSaving) {
    submitLabel = t('Saving...')
  } else if (isEdit) {
    submitLabel = t('Update')
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={isEdit ? t('Edit Vendor') : t('Create Vendor')}
      description={
        isEdit
          ? t('Update vendor information for {{name}}', {
              name: currentVendor?.name,
            })
          : t('Add a new vendor to the system')
      }
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
            disabled={isSaving}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='submit'
            form={VENDOR_MUTATE_FORM_ID}
            disabled={isSaving}
          >
            {isSaving ? (
              <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            ) : null}
            {submitLabel}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id={VENDOR_MUTATE_FORM_ID}
          onSubmit={form.handleSubmit(onSubmit)}
          className='space-y-4'
        >
          <FormField
            control={form.control}
            name='name'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Vendor Name *')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t('OpenAI, Anthropic, etc.')}
                    {...field}
                  />
                )}
                {(!id || !query.isPending) && !query.isError && (
                  <Form {...form}>
                    <form
                      id='vendor-mutate-form'
                      className={sideDrawerFormClassName()}
                      onSubmit={form.handleSubmit((values) =>
                        save.mutate(values)
                      )}
                    >
                      <FormField
                        control={form.control}
                        name='name'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Vendor Name *')}</FormLabel>
                            <FormControl>
                              <Input maxLength={128} {...field} />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name='description'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Description')}</FormLabel>
                            <FormControl>
                              <Textarea rows={4} {...field} />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name='icon'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Icon')}</FormLabel>
                            <FormControl>
                              <LobeIconField
                                value={field.value ?? ''}
                                onChange={field.onChange}
                              />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      {query.data && id && (
                        <div className='text-muted-foreground space-y-1 border-t pt-3 text-xs'>
                          <p>
                            {t('Created At')}:{' '}
                            {formatTimestampToDate(query.data.created_time)}
                          </p>
                          <p>
                            {t('Updated At')}:{' '}
                            {formatTimestampToDate(query.data.updated_time)}
                          </p>
                        </div>
                      )}
                      {save.isError && (
                        <ErrorState
                          description={vendorErrorMessage(save.error)}
                          action={
                            id ? (
                              <Button
                                variant='outline'
                                type='button'
                                onClick={async () => {
                                  const response = await query.refetch()
                                  if (response.data) {
                                    form.reset({
                                      name: response.data.name,
                                      description:
                                        response.data.description ?? '',
                                      icon: response.data.icon ?? '',
                                      version: response.data.version,
                                    })
                                    save.reset()
                                  }
                                }}
                              >
                                {t('Reload vendor details')}
                              </Button>
                            ) : undefined
                          }
                        />
                      )}
                    </form>
                  </Form>
                )}
              </div>
              <SheetFooter className={sideDrawerFooterClassName()}>
                <Button
                  variant='outline'
                  disabled={save.isPending}
                  onClick={() => close(false)}
                >
                  {t('Close')}
                </Button>
                <Button
                  type='submit'
                  form='vendor-mutate-form'
                  disabled={
                    save.isPending ||
                    Boolean(id && (query.isPending || query.isError))
                  }
                >
                  {save.isPending ? t('Saving...') : t('Save metadata')}
                </Button>
              </SheetFooter>
            </>
          )}
          {props.open && section === 'models' && query.data && (
            <VendorLinkedModels
              key={query.data.id}
              vendor={query.data}
              onNavigate={(navigate) => {
                pendingNavigation.current = navigate
                if (close(false)) {
                  pendingNavigation.current = null
                  navigate()
                }
              }}
            />
          )}
        </SheetContent>
      </Sheet>
      <ConfirmDialog
        open={confirmClose}
        onOpenChange={(open) => {
          setConfirmClose(open)
          if (!open) pendingNavigation.current = null
        }}
        title={t('Discard unsaved changes?')}
        desc={t('Your changes have not been saved.')}
        confirmText={t('Discard changes')}
        handleConfirm={() => {
          setConfirmClose(false)
          props.onOpenChange(false)
          pendingNavigation.current?.()
          pendingNavigation.current = null
        }}
      />
    </>
  )
}
