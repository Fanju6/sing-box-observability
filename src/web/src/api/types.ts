import type { components } from './schema'

type Schemas = components['schemas']

// API types are aliases of the OpenAPI-generated schema. Keep UI-only types
// below this block so contract drift is caught by TypeScript during builds.
export type HealthResponse = Schemas['HealthResponse']
export type SessionInfo = Schemas['SessionInfo']
export type SessionResponse = SessionInfo
export type LoginRequest = Schemas['LoginRequest']
export type SourceState = Schemas['SourceState']
export type SourceInfo = Schemas['SourceInfo']
export type RankingDimension = Schemas['RankingDimension']
export type RankingSort = Schemas['RankingSort']
export type Capabilities = Schemas['Capabilities']
export type CollectorInfo = Schemas['CollectorInfo']
export type CollectorChannel = Schemas['CollectorChannel']
export type CollectorChannels = Schemas['CollectorChannels']
export type MetaResponse = Schemas['MetaResponse']
export type StatusSnapshot = Schemas['StatusSnapshot']
export type TimeWindow = Schemas['TimeWindow']
export type RangeTotals = Schemas['RangeTotals']
export type TimePoint = Schemas['TimePoint']
export type SeriesPoint = TimePoint
export type DimensionTimePoint = Schemas['DimensionTimePoint']
export type DimensionSeriesResponse = Schemas['DimensionSeriesResponse']
export type UrlTestResult = Schemas['UrlTestResult']
export type CompactRanking = Schemas['CompactRanking']
export type OverviewResponse = Schemas['OverviewResponse']
export type RankingItem = Schemas['RankingItem']
export type RankingEntry = RankingItem
export type RankingsResponse = Schemas['RankingsResponse']
export type Connection = Schemas['Connection']
export type ConnectionPage = Schemas['ConnectionPage']
export type SourceStateEventData = Schemas['SourceStateEventData']
export type ResyncEventData = Schemas['ResyncEventData']
export type EventEnvelope = Schemas['EventEnvelope']
export type EventData = EventEnvelope['data']
export type EventType = EventEnvelope['type']
export type ErrorResponse = Schemas['ErrorResponse']

export type TimeRange = components['parameters']['Range']
export type TimeWindowSelection =
  | { range: TimeRange }
  | { from: string; to: string }

export interface ConnectionsQuery {
  tab: 'active' | 'recent'
  q?: string
  network?: string
  outbound?: string
  page: number
}
