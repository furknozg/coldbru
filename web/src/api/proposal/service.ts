import type { components } from './schema'
import { proposalApi } from '../client'
import { unwrap } from '../errors'

type Schemas = components['schemas']

export type CampaignInfo = Schemas['flow.CampaignInfo']
export type CampaignView = Schemas['flow.CampaignView']
export type VariantView = Schemas['flow.VariantView']
export type StepKind = Schemas['flow.StepInfo']['kind']

/** Page size of the campaign list; the backend allows 1 to 100 (D-072). */
export const CAMPAIGN_PAGE_SIZE = 20

const id = (value: string) => ({ params: { path: { id: value } } })

/** One page of campaigns, newest first; a page shorter than `limit` is the last. */
export async function listCampaigns(offset: number, limit = CAMPAIGN_PAGE_SIZE) {
  const page = await unwrap(proposalApi.GET('/campaigns', { params: { query: { limit, offset } } }))
  return page.items
}

export const getCampaign = (campaignId: string): Promise<CampaignView> =>
  unwrap(proposalApi.GET('/campaigns/{id}', id(campaignId)))

export const createCampaign = (brief: string): Promise<CampaignInfo> =>
  unwrap(proposalApi.POST('/campaigns', { body: { brief } }))

export const addContext = (campaignId: string, body: string) =>
  unwrap(proposalApi.POST('/campaigns/{id}/context', { ...id(campaignId), body: { body } }))

export const regenerateCampaign = (campaignId: string): Promise<CampaignView> =>
  unwrap(proposalApi.POST('/campaigns/{id}/regenerate', id(campaignId)))

export const reopenStep = (campaignId: string, kind: StepKind) =>
  unwrap(proposalApi.POST('/campaigns/{id}/reopen', { ...id(campaignId), body: { kind } }))

export const cancelCampaign = (campaignId: string, reason?: string): Promise<CampaignView> =>
  unwrap(
    proposalApi.POST('/campaigns/{id}/cancel', {
      ...id(campaignId),
      body: reason ? { reason } : {},
    }),
  )

export const confirmCampaign = (campaignId: string): Promise<CampaignView> =>
  unwrap(proposalApi.POST('/campaigns/{id}/confirm', id(campaignId)))

export const regenerateStep = (stepId: string): Promise<CampaignView> =>
  unwrap(proposalApi.POST('/steps/{id}/regenerate', id(stepId)))

export const approveVariant = (variantId: string): Promise<CampaignView> =>
  unwrap(proposalApi.POST('/variants/{id}/approve', id(variantId)))

export const rejectVariant = (variantId: string, reason: string): Promise<CampaignView> =>
  unwrap(proposalApi.POST('/variants/{id}/reject', { ...id(variantId), body: { reason } }))

export const editVariant = (variantId: string, content: Record<string, unknown>) =>
  unwrap(
    proposalApi.POST('/variants/{id}/edit', {
      ...id(variantId),
      body: { content: content as Record<string, never> },
    }),
  )

/**
 * LLM mode of the proposal service (`checks.llm.detail`, for example `mode=mock`) or `null` when
 * the service cannot be asked. A 503 readiness report still carries the check, so it is read too.
 */
export async function getLlmMode(): Promise<string | null> {
  try {
    const { data, error } = await proposalApi.GET('/readyz')
    const detail = (data ?? error)?.checks?.llm?.detail
    return detail?.replace(/^mode=/, '') || null
  } catch {
    return null
  }
}
