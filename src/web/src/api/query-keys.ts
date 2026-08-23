export const queryKeys = {
  session: ['session'] as const,
  meta: ['meta'] as const,
  overview: (range?: string) => ['overview', range ?? '1h'] as const,
  rankings: (params: { dimension: string; sort?: string; range?: string; from?: string; to?: string; limit?: number }) =>
    ['rankings', params.dimension, params.sort ?? 'traffic', params.range ?? '', params.from ?? '', params.to ?? '', params.limit ?? 10] as const,
  dimensionSeries: (params: { dimension: string; value: string; range?: string; from?: string; to?: string }) =>
    ['dimensions', params.dimension, params.value, params.range ?? '', params.from ?? '', params.to ?? ''] as const,
  activeConnections: (params?: { q?: string; network?: string; outbound?: string; offset?: number; limit?: number }) =>
    ['connections', 'active', params?.q ?? '', params?.network ?? '', params?.outbound ?? '', params?.offset ?? 0, params?.limit ?? 50] as const,
  recentConnections: (params?: { range?: string; q?: string; network?: string; outbound?: string; offset?: number; limit?: number }) =>
    ['connections', 'recent', params?.range ?? '1h', params?.q ?? '', params?.network ?? '', params?.outbound ?? '', params?.offset ?? 0, params?.limit ?? 50] as const,
}
