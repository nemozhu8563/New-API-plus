import type { Table } from '@tanstack/react-table'
import { Download } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import { Button } from '@/components/ui/button'

import type { Redemption } from '../types'

type DataTableBulkActionsProps<TData> = {
  table: Table<TData>
}

export function DataTableBulkActions<TData>({
  table,
}: DataTableBulkActionsProps<TData>) {
  const { t } = useTranslation()
  const selectedRows = table.getSelectedRowModel().rows

  const contentToCopy = useMemo(() => {
    const selectedCodes = selectedRows.map((row) => {
      const redemption = row.original as Redemption
      return `${redemption.name}\t${redemption.key}`
    })
    return selectedCodes.join('\n')
  }, [selectedRows])

  const handleExport = () => {
    const codes = selectedRows
      .map((row) => (row.original as Redemption).key)
      .join('\r\n')
    const blob = new Blob([`\ufeff${codes}`], {
      type: 'text/plain;charset=utf-8;',
    })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `redemption-codes-${new Date().toISOString().slice(0, 10)}.txt`
    link.click()
    setTimeout(() => URL.revokeObjectURL(url), 0)
  }

  return (
    <BulkActionsToolbar table={table} entityName={t('redemption code')}>
      <CopyButton
        value={contentToCopy}
        variant='outline'
        size='icon'
        className='size-8'
        tooltip={t('Copy selected codes')}
        successTooltip={t('Codes copied!')}
        aria-label={t('Copy selected codes')}
      />
      <Button
        variant='outline'
        size='icon'
        className='size-8'
        onClick={handleExport}
        title={t('Export selected codes')}
        aria-label={t('Export selected codes')}
      >
        <Download />
      </Button>
    </BulkActionsToolbar>
  )
}
