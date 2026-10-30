import type { components } from '@/api/proposal/schema'

type Schemas = components['schemas']

export type CampaignInfo = Schemas['flow.CampaignInfo']
export type CampaignView = Schemas['flow.CampaignView']
export type StepView = Schemas['flow.StepView']
export type VariantView = Schemas['flow.VariantView']
export type NoteView = Schemas['flow.NoteView']
export type Slot = Schemas['flow.Slot']

export type CampaignStatus = CampaignInfo['status']
export type StepKind = StepView['kind']
export type Channel = NonNullable<Slot['channel']>

/** Order of the proposal flow (D-033): a step opens when the one before it is approved. */
export const STEP_ORDER: readonly StepKind[] = ['audience', 'channel_plan', 'content', 'schedule']

export const CAMPAIGN_STATUSES: readonly CampaignStatus[] = [
  'drafting',
  'scheduled',
  'sending',
  'completed',
  'cancelled',
]

/** Limits the Proposal API enforces (D-072). */
export const MAX_BRIEF_CHARS = 2000
export const MAX_NOTE_CHARS = 2000
export const MAX_REASON_CHARS = 1000

/** Statuses a campaign can still be cancelled in (D-074). */
export const CANCELLABLE: readonly CampaignStatus[] = ['drafting', 'scheduled', 'sending']

/** Stable text key of a slot: the same slot always yields the same string. */
export function slotKey(slot: Slot): string {
  return [slot.language ?? '', slot.channel ?? '', slot.timezone ?? ''].join('|')
}
