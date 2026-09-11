import { Download, Loader2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'

import { exportUsageLogsCSV } from '../api'
import { buildUsageLogsExportParams, downloadUsageLogsCSV } from '../lib/export'
import type { LogCategory } from '../types'

interface UsageLogsExportButtonProps {
  logCategory: LogCategory
  isAdmin: boolean
  searchParams: Record<string, unknown>
  columnFilters: Array<{ id: string; value: unknown }>
}

export function UsageLogsExportButton(props: UsageLogsExportButtonProps) {
  const { t } = useTranslation()
  const [exporting, setExporting] = useState(false)

  const handleExport = async () => {
    if (exporting) return

    setExporting(true)
    try {
      const exported = await exportUsageLogsCSV({
        logCategory: props.logCategory,
        isAdmin: props.isAdmin,
        params: buildUsageLogsExportParams({
          logCategory: props.logCategory,
          isAdmin: props.isAdmin,
          searchParams: props.searchParams,
          columnFilters: props.columnFilters,
        }),
      })
      downloadUsageLogsCSV(exported)
      toast.success(t('Usage logs exported'))
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to export usage logs')
      )
    } finally {
      setExporting(false)
    }
  }

  return (
    <Button
      type='button'
      variant='outline'
      onClick={handleExport}
      disabled={exporting}
      aria-label={t('Export CSV')}
    >
      {exporting ? <Loader2 className='animate-spin' /> : <Download />}
      <span className='hidden sm:inline'>{t('Export CSV')}</span>
    </Button>
  )
}
