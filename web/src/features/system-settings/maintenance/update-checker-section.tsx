import { useTranslation } from 'react-i18next'

import { formatTimestamp } from '@/lib/format'

import { SettingsSection } from '../components/settings-section'

type UpdateCheckerSectionProps = {
  currentVersion?: string | null
  startTime?: number | null
}

export function UpdateCheckerSection({
  currentVersion,
  startTime,
}: UpdateCheckerSectionProps) {
  const { t } = useTranslation()

  const uptime = startTime ? formatTimestamp(startTime) : t('Unknown')
  const version = currentVersion || t('Unknown')

  return (
    <SettingsSection title={t('System maintenance')}>
      <div className='grid gap-4 md:grid-cols-2'>
        <div className='rounded-lg border p-4'>
          <div className='text-muted-foreground text-sm'>
            {t('Current version')}
          </div>
          <div className='text-lg font-semibold'>{version}</div>
        </div>
        <div className='rounded-lg border p-4'>
          <div className='text-muted-foreground text-sm'>
            {t('Uptime since')}
          </div>
          <div className='text-lg font-semibold'>{uptime}</div>
        </div>
      </div>
    </SettingsSection>
  )
}
