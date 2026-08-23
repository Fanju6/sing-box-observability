import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query'
import { api, queryKeys } from '@/api'
import type {
  TimeRange,
  SessionResponse,
  MetaResponse,
  OverviewResponse,
  RankingsResponse,
  ConnectionPage,
  DimensionSeriesResponse,
} from '@/api/types'

function rangeToParams(range?: TimeRange, from?: string, to?: string) {
  if (from && to) return { from, to }
  if (range) return { range }
  return { range: '1h' as const }
}

export function useSession() {
  return useQuery<SessionResponse>({
    queryKey: queryKeys.session,
    queryFn: () => api.getSession(),
    staleTime: 30_000,
  })
}

export function useLogin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (token: string) => api.login(token),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.session }),
  })
}

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api.logout(),
    onSuccess: () => {
      qc.clear()
      qc.setQueryData(queryKeys.session, { authEnabled: true, authenticated: false } satisfies SessionResponse)
    },
  })
}

export function useMeta() {
  return useQuery<MetaResponse>({
    queryKey: queryKeys.meta,
    queryFn: () => api.getMeta(),
    staleTime: 10_000,
    refetchInterval: 15_000,
  })
}

export function useOverview(range?: TimeRange, from?: string, to?: string) {
  return useQuery<OverviewResponse>({
    queryKey: queryKeys.overview(range ?? (from && to ? `${from}-${to}` : '1h')),
    queryFn: () => api.getOverview(rangeToParams(range, from, to)),
    refetchInterval: 5000,
    placeholderData: (prev) => prev,
  })
}

export function useRankings(params: {
  dimension: string
  sort?: string
  range?: TimeRange
  from?: string
  to?: string
  limit?: number
  enabled?: boolean
}) {
  return useQuery<RankingsResponse>({
    queryKey: queryKeys.rankings({ dimension: params.dimension, sort: params.sort, range: params.range, from: params.from, to: params.to, limit: params.limit }),
    queryFn: () =>
      api.getRankings({
        dimension: params.dimension,
        sort: params.sort ?? 'traffic',
        limit: params.limit ?? 10,
        ...rangeToParams(params.range, params.from, params.to),
      }),
    enabled: params.enabled ?? true,
    staleTime: 30_000,
    placeholderData: (prev) => prev,
    retry: (_, err) => {
      const apiErr = err as { code?: string; status?: number }
      if (apiErr.code === 'SENSITIVE_DIMENSION_DISABLED') return false
      if (apiErr.status === 403) return false
      return true
    },
  })
}

export function useTrends(range?: TimeRange, from?: string, to?: string) {
  return useOverview(range, from, to)
}

export function useDimensionSeries(params: {
  dimension: 'network' | 'inbound' | 'outbound'
  value: string
  range?: TimeRange
  from?: string
  to?: string
}) {
  return useQuery<DimensionSeriesResponse>({
    queryKey: queryKeys.dimensionSeries(params),
    queryFn: () => api.getDimensionSeries({
      dimension: params.dimension,
      value: params.value,
      ...rangeToParams(params.range, params.from, params.to),
    }),
    enabled: params.value.length > 0,
    staleTime: 15_000,
    placeholderData: (previous) => previous,
  })
}

export function useActiveConnections(params?: {
  q?: string
  network?: string
  outbound?: string
  page?: number
  limit?: number
}) {
  const offset = ((params?.page ?? 1) - 1) * (params?.limit ?? 50)
  return useQuery<ConnectionPage>({
    queryKey: queryKeys.activeConnections({
      q: params?.q,
      network: params?.network,
      outbound: params?.outbound,
      offset,
      limit: params?.limit ?? 50,
    }),
    queryFn: () =>
      api.getActiveConnections({
        q: params?.q || undefined,
        network: params?.network || undefined,
        outbound: params?.outbound || undefined,
        limit: params?.limit ?? 50,
        offset,
      }),
    refetchInterval: 3000,
    placeholderData: (prev) => prev,
  })
}

export function useRecentConnections(params?: {
  range?: TimeRange
  q?: string
  network?: string
  outbound?: string
  page?: number
  limit?: number
}) {
  const offset = ((params?.page ?? 1) - 1) * (params?.limit ?? 50)
  return useQuery<ConnectionPage>({
    queryKey: queryKeys.recentConnections({
      range: params?.range ?? '1h',
      q: params?.q,
      network: params?.network,
      outbound: params?.outbound,
      offset,
      limit: params?.limit ?? 50,
    }),
    queryFn: () =>
      api.getRecentConnections({
        ...rangeToParams(params?.range),
        q: params?.q || undefined,
        network: params?.network || undefined,
        outbound: params?.outbound || undefined,
        limit: params?.limit ?? 50,
        offset,
      }),
    staleTime: 30_000,
    placeholderData: (prev) => prev,
  })
}
