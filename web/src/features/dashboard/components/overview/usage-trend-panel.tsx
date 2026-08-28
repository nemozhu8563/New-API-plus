import { useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Area,
  CartesianGrid,
  ComposedChart,
  Line,
  XAxis,
  YAxis,
} from 'recharts'

import { Button } from '@/components/ui/button'
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'
import { Skeleton } from '@/components/ui/skeleton'
import { getUserQuotaDates } from '@/features/dashboard/api'
import type { QuotaDataItem } from '@/features/dashboard/types'
import { toIntlLocale } from '@/i18n/languages'
import {
  formatCompactNumber,
  formatNumber,
  formatQuota,
  quotaUnitsToDollars,
} from '@/lib/format'
import {
  dateToUnixTimestamp,
  formatDate,
  getNormalizedDateRange,
} from '@/lib/time'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import { PanelWrapper } from '../ui/panel-wrapper'

type UsageTrendRange = 7 | 30

const SECONDS_PER_DAY = 24 * 60 * 60

interface UsageTrendPoint {
  timestamp: number
  date: string
  requests: number
  quota: number
}

function toUsageNumber(value: number | undefined): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? Math.max(0, parsed) : 0
}

function buildUsageTrendPoints(
  items: QuotaDataItem[],
  startTimestamp: number,
  days: UsageTrendRange
): UsageTrendPoint[] {
  const startDate = new Date(startTimestamp * 1000)
  const points = Array.from({ length: days }, (_, index) => {
    const date = new Date(startDate)
    date.setDate(startDate.getDate() + index)
    const timestamp = dateToUnixTimestamp(date)
    return {
      timestamp,
      date: formatDate(timestamp),
      requests: 0,
      quota: 0,
    }
  })
  const pointsByDate = new Map(points.map((point) => [point.date, point]))

  for (const item of items) {
    const point = pointsByDate.get(formatDate(item.created_at))
    if (!point) continue
    point.requests += toUsageNumber(item.count)
    point.quota += toUsageNumber(item.quota)
  }

  return points
}

export function UsageTrendPanel() {
  const { i18n, t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const [rangeDays, setRangeDays] = useState<UsageTrendRange>(7)
  const timeRange = useMemo(() => {
    const range = getNormalizedDateRange(rangeDays - 1)
    const endTimestamp = dateToUnixTimestamp(range.end)
    const maximumRangeSeconds = rangeDays * SECONDS_PER_DAY
    return {
      start_timestamp: Math.max(
        dateToUnixTimestamp(range.start),
        endTimestamp - maximumRangeSeconds + 1
      ),
      end_timestamp: endTimestamp,
    }
  }, [rangeDays])

  const usageQuery = useQuery({
    queryKey: [
      'dashboard',
      'overview',
      'usage-trend',
      user?.id,
      rangeDays,
      timeRange.start_timestamp,
      timeRange.end_timestamp,
    ],
    queryFn: () =>
      getUserQuotaDates(
        {
          ...timeRange,
          default_time: 'day',
        },
        false
      ),
    enabled: Boolean(user),
    staleTime: 60 * 1000,
    retry: false,
  })

  const sourceItems = usageQuery.data?.data
  const points = useMemo(
    () =>
      buildUsageTrendPoints(
        usageQuery.data?.success ? (sourceItems ?? []) : [],
        timeRange.start_timestamp,
        rangeDays
      ),
    [
      rangeDays,
      sourceItems,
      timeRange.start_timestamp,
      usageQuery.data?.success,
    ]
  )
  const chartData = useMemo(() => {
    const locale = toIntlLocale(i18n.resolvedLanguage ?? i18n.language)
    const labelOptions: Intl.DateTimeFormatOptions =
      rangeDays === 7
        ? { weekday: 'short' }
        : { month: 'numeric', day: 'numeric' }
    const labelFormatter = new Intl.DateTimeFormat(locale, labelOptions)

    return points.map((point, index) => ({
      ...point,
      label:
        index === points.length - 1
          ? t('Today')
          : labelFormatter.format(new Date(point.timestamp * 1000)),
      cost: quotaUnitsToDollars(point.quota),
    }))
  }, [i18n.language, i18n.resolvedLanguage, points, rangeDays, t])
  const totalRequests = points.reduce(
    (total, point) => total + point.requests,
    0
  )
  const totalQuota = points.reduce((total, point) => total + point.quota, 0)
  const hasUsage = totalRequests > 0 || totalQuota > 0
  const chartConfig = useMemo<ChartConfig>(
    () => ({
      requests: {
        label: t('Requests'),
        color: 'var(--chart-1)',
      },
      cost: {
        label: t('Cost'),
        color: 'var(--chart-4)',
      },
    }),
    [t]
  )

  return (
    <PanelWrapper
      title={t('Usage trend')}
      description={t(rangeDays === 7 ? '7 Days' : '30 Days')}
      className='h-full min-w-0'
      contentClassName='pb-3'
      headerActions={
        <div
          role='group'
          aria-label={t('Period')}
          className='bg-muted flex shrink-0 rounded-lg p-0.5'
        >
          {([7, 30] as const).map((days) => {
            const selected = rangeDays === days
            return (
              <Button
                key={days}
                type='button'
                variant={selected ? 'outline' : 'ghost'}
                size='sm'
                aria-pressed={selected}
                onClick={() => setRangeDays(days)}
                className={cn(
                  'h-7 min-w-16 rounded-md px-2.5 text-xs shadow-none',
                  selected && 'border-primary text-primary bg-card'
                )}
              >
                {t(days === 7 ? '7 Days' : '30 Days')}
              </Button>
            )
          })}
        </div>
      }
    >
      {usageQuery.isLoading && (
        <div className='space-y-4'>
          <div className='flex gap-6'>
            <Skeleton className='h-8 w-36' />
            <Skeleton className='h-8 w-32' />
          </div>
          <Skeleton className='h-72 w-full' />
        </div>
      )}
      {usageQuery.isError && (
        <div className='text-muted-foreground flex h-[20rem] items-center justify-center text-sm'>
          {t('Failed to load usage trend')}
        </div>
      )}
      {!usageQuery.isLoading && !usageQuery.isError && (
        <>
          <div className='flex flex-wrap items-center gap-x-7 gap-y-2'>
            <div
              role='group'
              aria-label={t('Requests')}
              className='flex items-baseline gap-2'
            >
              <span className='bg-chart-1 size-2 rounded-full' />
              <span className='font-mono text-lg font-semibold tabular-nums'>
                {formatNumber(totalRequests)}
              </span>
              <span className='text-muted-foreground text-xs'>
                {t('Requests')}
              </span>
            </div>
            <div
              role='group'
              aria-label={t('Cost')}
              className='flex items-baseline gap-2'
            >
              <span className='bg-chart-4 size-2 rounded-full' />
              <span className='font-mono text-lg font-semibold tabular-nums'>
                {formatQuota(totalQuota)}
              </span>
              <span className='text-muted-foreground text-xs'>{t('Cost')}</span>
            </div>
          </div>

          {hasUsage ? (
            <ChartContainer
              config={chartConfig}
              className='mt-3 h-[18rem] w-full sm:h-72'
              initialDimension={{ width: 720, height: 288 }}
            >
              <ComposedChart
                accessibilityLayer
                data={chartData}
                margin={{ top: 8, right: 4, bottom: 0, left: 0 }}
              >
                <CartesianGrid vertical={false} strokeDasharray='3 3' />
                <XAxis
                  dataKey='label'
                  axisLine={false}
                  tickLine={false}
                  tickMargin={10}
                  minTickGap={24}
                  interval='preserveStartEnd'
                />
                <YAxis
                  yAxisId='requests'
                  axisLine={false}
                  tickLine={false}
                  tickMargin={8}
                  width={44}
                  tickFormatter={(value) => formatCompactNumber(Number(value))}
                />
                <YAxis
                  yAxisId='cost'
                  orientation='right'
                  axisLine={false}
                  tickLine={false}
                  tickMargin={8}
                  width={36}
                  tickFormatter={(value) => formatCompactNumber(Number(value))}
                />
                <ChartTooltip
                  cursor={false}
                  content={<ChartTooltipContent indicator='line' />}
                />
                <Area
                  yAxisId='cost'
                  type='monotone'
                  dataKey='cost'
                  stroke='var(--color-cost)'
                  fill='var(--color-cost)'
                  fillOpacity={0.12}
                  strokeWidth={2}
                  dot={false}
                  activeDot={{ r: 4 }}
                  isAnimationActive={false}
                />
                <Line
                  yAxisId='requests'
                  type='monotone'
                  dataKey='requests'
                  stroke='var(--color-requests)'
                  strokeWidth={2}
                  dot={{ r: 2.5, strokeWidth: 2 }}
                  activeDot={{ r: 4 }}
                  isAnimationActive={false}
                />
              </ComposedChart>
            </ChartContainer>
          ) : (
            <div className='text-muted-foreground flex h-[20rem] items-center justify-center text-sm'>
              {t('No data available')}
            </div>
          )}

          <table className='sr-only' aria-label={t('Usage trend')}>
            <thead>
              <tr>
                <th>{t('Time')}</th>
                <th>{t('Requests')}</th>
                <th>{t('Cost')}</th>
              </tr>
            </thead>
            <tbody>
              {points.map((point) => (
                <tr key={point.date}>
                  <td>{point.date}</td>
                  <td>{formatNumber(point.requests)}</td>
                  <td>{formatQuota(point.quota)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </PanelWrapper>
  )
}
