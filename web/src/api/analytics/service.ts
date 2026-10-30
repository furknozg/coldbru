import { analyticsApi } from '../client'
import { ApiError, unwrap } from '../errors'
import type { components } from './schema'

export type CampaignReport = components['schemas']['api.CampaignReport']

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
