import type { PingStatus } from '@/features/dashboard/types'

export function resolvePrimaryApiAddress(
  serverAddress: unknown,
  currentOrigin: string
): string {
  const configuredAddress =
    typeof serverAddress === 'string' ? serverAddress.trim() : ''
  const baseAddress = (configuredAddress || currentOrigin.trim()).replace(
    /\/+$/,
    ''
  )

  return baseAddress.endsWith('/v1') ? baseAddress : `${baseAddress}/v1`
}

/**
 * Get color class for latency status
 */
export function getLatencyColorClass(latency: number): string {
  if (latency < 200) {
    return 'text-green-600 dark:text-green-400'
  }
  if (latency < 500) {
    return 'text-yellow-600 dark:text-yellow-400'
  }
  return 'text-red-600 dark:text-red-400'
}

/**
 * Test URL latency
 */
export async function testUrlLatency(url: string): Promise<PingStatus> {
  try {
    const startTime = performance.now()
    await fetch(url, {
      method: 'HEAD',
      mode: 'no-cors',
      cache: 'no-cache',
    })
    const endTime = performance.now()
    const latency = Math.round(endTime - startTime)

    return { latency, testing: false, error: false }
  } catch {
    return { latency: null, testing: false, error: true }
  }
}

/**
 * Open external speed test link
 */
export function openExternalSpeedTest(url: string): void {
  const encodedUrl = encodeURIComponent(url)
  const speedTestUrl = `https://www.tcptest.cn/http/${encodedUrl}`
  window.open(speedTestUrl, '_blank', 'noopener,noreferrer')
}

/**
 * Get default ping status
 */
export function getDefaultPingStatus(): PingStatus {
  return {
    latency: null,
    testing: false,
    error: false,
  }
}
