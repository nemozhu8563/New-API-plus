import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const sensitiveSchema = z.object({
  CheckSensitiveEnabled: z.boolean(),
  CheckSensitiveOnPromptEnabled: z.boolean(),
  SensitiveWordsHighRisk: z.string(),
  SensitiveWordsAudit: z.string(),
  SensitiveWords: z.string(),
})

type SensitiveFormValues = z.infer<typeof sensitiveSchema>

const sensitiveOptionKeys = [
  'SensitiveWordsHighRisk',
  'SensitiveWordsAudit',
  'SensitiveWords',
  'CheckSensitiveOnPromptEnabled',
  'CheckSensitiveEnabled',
] as const satisfies readonly (keyof SensitiveFormValues)[]

type SensitiveWordsSectionProps = {
  defaultValues: SensitiveFormValues
}

export function SensitiveWordsSection({
  defaultValues,
}: SensitiveWordsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const form = useForm<SensitiveFormValues>({
    resolver: zodResolver(sensitiveSchema),
    defaultValues,
  })

  useEffect(() => {
    form.reset(defaultValues)
  }, [defaultValues, form])

  const onSubmit = async (values: SensitiveFormValues) => {
    for (const key of sensitiveOptionKeys) {
      if (values[key] === defaultValues[key]) continue
      const result = await updateOption.mutateAsync({
        key,
        value: values[key],
      })
      if (!result.success) return
    }
  }

  return (
    <SettingsSection title={t('Sensitive Words')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save sensitive words'
          />
          <div className='space-y-4'>
            <FormField
              control={form.control}
              name='CheckSensitiveEnabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Enable filtering')}</FormLabel>
                    <FormDescription>
                      {t('Applies blocking and audit-only keyword policies.')}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />

            <FormField
              control={form.control}
              name='CheckSensitiveOnPromptEnabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Inspect user prompts')}</FormLabel>
                    <FormDescription>
                      {t(
                        'When enabled, prompts are scanned before reaching upstream models.'
                      )}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
          </div>

          <div className='grid gap-6 lg:grid-cols-3'>
            <FormField
              control={form.control}
              name='SensitiveWordsHighRisk'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('High-risk blocked keywords')}</FormLabel>
                  <FormControl>
                    <Textarea
                      rows={14}
                      placeholder={t('Enter one keyword per line')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Blocks sexual violence, child sexual exploitation, self-harm instructions, weapons, and explosives.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='SensitiveWordsAudit'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Audit-only keywords')}</FormLabel>
                  <FormControl>
                    <Textarea
                      rows={14}
                      placeholder={t('Enter one keyword per line')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Logs matches for review without blocking the request.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='SensitiveWords'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('NSFW blocked keywords')}</FormLabel>
                  <FormControl>
                    <Textarea
                      rows={14}
                      placeholder={t('Enter one keyword per line')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Blocks explicit sexual-content requests.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='text-muted-foreground space-y-1 text-sm'>
            <p>
              {t(
                'Each line represents one keyword. Leave blank to disable the list but keep the switch states.'
              )}
            </p>
            <p>
              {t(
                'If a keyword appears in multiple lists, blocking takes priority over audit-only.'
              )}
            </p>
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
