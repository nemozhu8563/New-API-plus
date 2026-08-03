import { api } from '@/lib/api'

import type {
  GetLogsParams,
  GetLogsResponse,
  GetLogStatsParams,
  GetLogStatsResponse,
  GetMidjourneyLogsParams,
  GetTaskLogsParams,
  LogCategory,
  UserInfo,
} from './types'

// ============================================================================
// Generic API Helpers
// ============================================================================

function buildQueryParams(params: Record<string, unknown>): URLSearchParams {
  const queryParams = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== '') {
      queryParams.append(key, String(value))
    }
  }
  return queryParams
}

function buildApiPath(endpoint: string, isAdmin: boolean): string {
  return isAdmin ? endpoint : `${endpoint}/self`
}

async function fetchLogs<T>(
  endpoint: string,
  params: T,
  isAdmin: boolean
): Promise<GetLogsResponse> {
  const paramRecord = params as unknown as Record<string, unknown>
  const queryParams = buildQueryParams({
    p: paramRecord.p || 1,
    page_size: paramRecord.page_size || 20,
    ...params,
  })
  const path = buildApiPath(endpoint, isAdmin)
  const res = await api.get(`${path}?${queryParams}`)
  return res.data
}

async function fetchLogStats<T>(
  endpoint: string,
  params: T,
  isAdmin: boolean
): Promise<GetLogStatsResponse> {
  const queryParams = buildQueryParams(
    params as unknown as Record<string, unknown>
  )
  const path = buildApiPath(endpoint, isAdmin)
  const res = await api.get(`${path}/stat?${queryParams}`)
  return res.data
}

// ============================================================================
// Common Log APIs
// ============================================================================

export const getAllLogs = (params: GetLogsParams = {}) =>
  fetchLogs('/api/log', params, true)

export const getUserLogs = (
  params: Omit<GetLogsParams, 'username' | 'channel'> = {}
) => fetchLogs('/api/log', params, false)

export const getLogStats = (params: GetLogStatsParams = {}) =>
  fetchLogStats('/api/log', params, true)

export const getUserLogStats = (
  params: Omit<GetLogStatsParams, 'username' | 'channel'> = {}
) => fetchLogStats('/api/log', params, false)

export async function getUserInfo(
  userId: number
): Promise<{ success: boolean; message?: string; data?: UserInfo }> {
  const res = await api.get(`/api/user/${userId}`)
  return res.data
}

// ============================================================================
// MjProxy (Drawing) Logs API
// ============================================================================

export const getAllMidjourneyLogs = (params: GetMidjourneyLogsParams) =>
  fetchLogs('/api/mj', params, true)

export const getUserMidjourneyLogs = (params: GetMidjourneyLogsParams) =>
  fetchLogs('/api/mj', params, false)

// ============================================================================
// Task Logs API
// ============================================================================

export const getAllTaskLogs = (params: GetTaskLogsParams) =>
  fetchLogs('/api/task', params, true)

export const getUserTaskLogs = (params: GetTaskLogsParams) =>
  fetchLogs('/api/task', params, false)

// ============================================================================
// CSV Export
// ============================================================================

const exportEndpoints: Record<LogCategory, string> = {
  common: '/api/log',
  drawing: '/api/mj',
  task: '/api/task',
}

export interface UsageLogsCSVExport {
  blob: Blob
  filename: string
}

function getExportFilename(
  contentDisposition: string | undefined,
  logCategory: LogCategory
): string {
  const encodedFilename = contentDisposition?.match(
    /filename\*=UTF-8''([^;]+)/i
  )
  if (encodedFilename?.[1]) {
    return decodeURIComponent(encodedFilename[1])
  }

  const plainFilename = contentDisposition?.match(
    /filename=(?:"([^"]+)"|([^;]+))/i
  )
  const filename = plainFilename?.[1] || plainFilename?.[2]?.trim()
  if (filename) return filename

  return `usage-logs-${logCategory}-${new Date().toISOString().slice(0, 10)}.csv`
}

async function getBlobErrorMessage(error: unknown): Promise<string | null> {
  if (typeof error !== 'object' || error === null || !('response' in error)) {
    return null
  }

  const response = error.response as { data?: unknown } | undefined
  const data = response?.data
  if (!(data instanceof Blob)) return null

  try {
    const payload = JSON.parse(await data.text()) as { message?: unknown }
    return typeof payload.message === 'string' ? payload.message : null
  } catch {
    return null
  }
}

export async function exportUsageLogsCSV(config: {
  logCategory: LogCategory
  isAdmin: boolean
  params: Record<string, unknown>
}): Promise<UsageLogsCSVExport> {
  const basePath = exportEndpoints[config.logCategory]
  const path = config.isAdmin ? `${basePath}/export` : `${basePath}/self/export`

  try {
    const response = await api.get<Blob>(path, {
      params: config.params,
      responseType: 'blob',
      disableDuplicate: true,
      skipBusinessError: true,
      skipErrorHandler: true,
    })
    return {
      blob: response.data,
      filename: getExportFilename(
        response.headers['content-disposition'],
        config.logCategory
      ),
    }
  } catch (error) {
    const message = await getBlobErrorMessage(error)
    if (message) throw new Error(message)
    throw error
  }
}
