import { useQuery } from '@tanstack/react-query'

import { requireServerSuccess } from '@/lib/server-error-message'

import { getRankings } from '../api'
import type { RankingPeriod } from '../types'

export function useRankings(period: RankingPeriod) {
  return useQuery({
    queryKey: ['rankings', period],
    queryFn: async () => requireServerSuccess(await getRankings(period)),
    staleTime: 5 * 60 * 1000,
  })
}
