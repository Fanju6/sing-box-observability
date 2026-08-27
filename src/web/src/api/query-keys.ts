export const queryKeys = {
  session: ['session'] as const,
  meta: ['meta'] as const,
  overview: (params?: { range?: string; from?: string; to?: string }) =>
    ['overview', params?.range ?? '', params?.from ?? '', params?.to ?? ''] as const,
  rankings: (params: { dimension: string; sort?: string; range?: string; from?: string; to?: string; limit?: number }) =>
    ['rankings', params.dimension, params.sort ?? 'traffic', params.range ?? '', params.from ?? '', params.to ?? '', params.limit ?? 10] as const,
  dimensionSeries: (params: { dimension: string; value: string; range?: string; from?: string; to?: string }) =>
    ['dimensions', params.dimension, params.value, params.range ?? '', params.from ?? '', params.to ?? ''] as const,
  activeConnections: (params?: { q?: string; network?: string; outbound?: string; offset?: number; limit?: number }) =>
    ['connections', 'active', params?.q ?? '', params?.network ?? '', params?.outbound ?? '', params?.offset ?? 0, params?.limit ?? 50] as const,
  recentConnections: (params?: { range?: string; from?: string; to?: string; q?: string; network?: string; outbound?: string; offset?: number; limit?: number }) =>
    ['connections', 'recent', params?.range ?? '', params?.from ?? '', params?.to ?? '', params?.q ?? '', params?.network ?? '', params?.outbound ?? '', params?.offset ?? 0, params?.limit ?? 50] as const,
}
