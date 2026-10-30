import type { components } from './schema'
import { proposalApi } from '../client'
import { unwrap } from '../errors'

type Schemas = components['schemas']

export type CampaignInfo = Schemas['flow.CampaignInfo']
export type CampaignView = Schemas['flow.CampaignView']
export type VariantView = Schemas['flow.VariantView']
export type ChatMessageView = Schemas['flow.ChatMessageView']
export type EmailPreview = Schemas['flow.EmailPreview']
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

/** Answers a question of the agent (D-088); the step is generated again with the answer. */
export const answerQuestion = (questionId: string, answer: string): Promise<CampaignView> =>
  unwrap(proposalApi.POST('/messages/{id}/answer', { ...id(questionId), body: { answer } }))

/**
 * Renders an email variant as it is sent (D-090): the stored blocks, or `content`, unsaved edited
 * blocks, which are validated like an edit; a 422 carries the problems. Nothing is stored.
 */
export const previewVariant = (
  variantId: string,
  content?: Record<string, unknown>,
): Promise<EmailPreview> =>
  unwrap(
    proposalApi.POST('/variants/{id}/preview', {
      ...id(variantId),
      ...(content ? { body: { content: content as Record<string, never> } } : {}),
    }),
  )
