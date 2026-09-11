import { useStatus } from '@/hooks/use-status'

import type { AnnouncementItem, FAQItem } from '../types'

/**
 * Get specific list from status data
 */
export function useStatusData<T = unknown>(
  enabledKey: string,
  dataKey: string
): { items: T[]; loading: boolean } {
  const { status, loading } = useStatus()
  const enabled = status ? status[enabledKey] !== false : false
  const items = (enabled ? status?.[dataKey] || [] : []) as T[]

  return { items, loading }
}

/**
 * Get announcements list
 */
export function useAnnouncements() {
  return useStatusData<AnnouncementItem>(
    'announcements_enabled',
    'announcements'
  )
}

/**
 * Get FAQ list
 */
export function useFAQ() {
  return useStatusData<FAQItem>('faq_enabled', 'faq')
}

/**
 * Get dashboard status-derived display data
 */
export function useDashboardStatus() {
  const { status } = useStatus()
  const hasStatus = Boolean(status)
  const serverAddress =
    status?.server_address ??
    status?.serverAddress ??
    status?.data?.server_address ??
    status?.data?.serverAddress

  return {
    serverAddress: typeof serverAddress === 'string' ? serverAddress : '',
    announcements: hasStatus && status?.announcements_enabled !== false,
    faq: hasStatus && status?.faq_enabled !== false,
    uptimeKuma: hasStatus && status?.uptime_kuma_enabled !== false,
  }
}
