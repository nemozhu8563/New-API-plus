/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
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
