import type { LogCategory } from '../types'
import { buildApiParams, buildBaseParams } from './utils'

interface BuildUsageLogsExportParamsConfig {
  logCategory: LogCategory
  isAdmin: boolean
  searchParams: Record<string, unknown>
  columnFilters: Array<{ id: string; value: unknown }>
}

function withoutPagination(
  params: Record<string, unknown>
): Record<string, unknown> {
  const { p: _page, page_size: _pageSize, ...filters } = params
  return filters
}

export function buildUsageLogsExportParams(
  config: BuildUsageLogsExportParamsConfig
): Record<string, unknown> {
  if (config.logCategory === 'common') {
    return withoutPagination(
      buildApiParams({
        page: 1,
        pageSize: 1,
        searchParams: config.searchParams,
        columnFilters: config.columnFilters,
        isAdmin: config.isAdmin,
      }) as unknown as Record<string, unknown>
    )
  }

  const baseParams = withoutPagination(
    buildBaseParams({
      page: 1,
      pageSize: 1,
      searchParams: config.searchParams,
      useMilliseconds: config.logCategory === 'drawing',
    })
  )
  if (!config.isAdmin) delete baseParams.channel_id

  if (config.searchParams.filter) {
    const filterName = config.logCategory === 'drawing' ? 'mj_id' : 'task_id'
    baseParams[filterName] = String(config.searchParams.filter)
  }

  return baseParams
}

export function downloadUsageLogsCSV(exported: {
  blob: Blob
  filename: string
}): void {
  const url = URL.createObjectURL(exported.blob)
  const link = document.createElement('a')
  link.href = url
  link.download = exported.filename
  link.click()
  setTimeout(() => URL.revokeObjectURL(url), 0)
}
