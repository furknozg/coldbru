import { analyticsApi } from '../client'
import { ApiError, unwrap } from '../errors'
import type { components } from './schema'

type Schemas = components['schemas']

export type CampaignReport = Schemas['api.CampaignReport']
export type Overview = Schemas['api.Overview']
export type OverviewSeries = Schemas['api.OverviewSeries']
export type OverviewSeriesPoint = Schemas['api.OverviewSeriesPoint']
export type LlmSeries = Schemas['api.LLMSeries']
export type LlmSeriesPoint = Schemas['api.LLMSeriesPoint']
export type PromptVersionUsage = Schemas['api.PromptVersionUsage']
export type DeliveryCount = Schemas['api.DeliveryCount']
export type PurposeUsage = Schemas['api.PurposeUsage']
export type ModelUsage = Schemas['api.ModelUsage']

/** Half-open range [from, to) of RFC 3339 UTC strings; a missing bound is open-ended (D-097). */
export interface TimeRange {
  from?: string
  to?: string
}

/** Both bounds and the bucket size, as the two series endpoints require (D-097). */
export interface SeriesQuery {
  from: string
  to: string
  bucket: 'hour' | 'day'
}

/** The report of a campaign, or `null` while Analytics has none yet (404, D-083). */
export async function getCampaignSummary(campaignId: string): Promise<CampaignReport | null> {
  try {
    return await unwrap(
      analyticsApi.GET('/campaigns/{id}/summary', { params: { path: { id: campaignId } } }),
    )
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) return null
    throw error
  }
}

export const getOverview = (range: TimeRange = {}): Promise<Overview> =>
  unwrap(analyticsApi.GET('/overview', { params: { query: range } }))

export const getOverviewSeries = (query: SeriesQuery): Promise<OverviewSeries> =>
  unwrap(analyticsApi.GET('/overview/series', { params: { query } }))

/** One row per prompt name and version over the calls of the range (D-060, D-083, D-097). */
export const getPromptVersionUsage = (range: TimeRange = {}): Promise<PromptVersionUsage[]> =>
  unwrap(
    analyticsApi.GET('/llm-usage', { params: { query: { group: 'prompt_version', ...range } } }),
  )

export const getLlmSeries = (query: SeriesQuery): Promise<LlmSeries> =>
  unwrap(analyticsApi.GET('/llm-usage/series', { params: { query } }))
