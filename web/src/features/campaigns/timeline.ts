import type { ProposalEvents } from '@/api/events'

import type { Labels, Translator } from './labels'
import type { CampaignView, Slot, StepKind } from './types'
import { slotKey } from './types'

/** Events that become timeline cards. The rest (ping, snapshot, content stream) never do. */
export const TIMELINE_EVENTS = [
  'screen_result',
  'step_opened',
  'step_approved',
  'step_superseded',
  'generation_started',
  'generation_failed',
  'variant_created',
  'variant_approved',
  'variant_rejected',
  'context_added',
  'campaign_status',
  'campaign_confirmed',
  'campaign_cancelled',
] as const

export type TimelineEventName = (typeof TIMELINE_EVENTS)[number]

/** An event received on this connection, kept so that facts the view does not carry survive. */
export type LiveEntry = {
  [K in TimelineEventName]: { name: K; data: ProposalEvents[K] }
}[TimelineEventName]

export type TimelineKind =
  | 'brief'
  | 'note'
  | 'rejection'
  | 'screen'
  | 'stepOpened'
  | 'generationStarted'
  | 'generationFailed'
  | 'variantCreated'
  | 'variantApproved'
  | 'stepApproved'
  | 'stepSuperseded'
  | 'campaignConfirmed'
  | 'campaignStatus'
  | 'campaignCancelled'

export interface TimelineItem {
  /** Same fact, same key: a live event and the view that now contains it are one card. */
  key: string
  at: string
  kind: TimelineKind
  /** `user` cards are what the marketer wrote; they are the only ones drawn as messages. */
  author: 'user' | 'system'
  tone: 'neutral' | 'success' | 'warning' | 'danger'
  title: string
  /** Plain text, shown with its line breaks; never markup. */
  body?: string
  /** The step the card is about; a click focuses it in the step panel. */
  stepId?: string
}

/** Tie-break for cards with the same instant: the order things happen in. */
const RANK: Record<TimelineKind, number> = {
  brief: 0,
  note: 1,
  screen: 2,
  stepOpened: 3,
  generationStarted: 4,
  generationFailed: 5,
  variantCreated: 6,
  rejection: 7,
  variantApproved: 8,
  stepApproved: 9,
  stepSuperseded: 10,
  campaignConfirmed: 11,
  campaignStatus: 12,
  campaignCancelled: 13,
}

const instant = (at: string) => {
  const ms = Date.parse(at)
  return Number.isNaN(ms) ? 0 : ms
}

/**
 * The cards of the centre column (D-087): the marketer's messages and system cards, oldest first.
 * History comes from the view, so a campaign opened later still tells its story; what only an event
 * knows (screening, generation start and failure, supersession, sending) comes from `live`.
 */
export function buildTimeline(
  view: CampaignView | null,
  live: readonly LiveEntry[],
  tr: Translator,
  labels: Labels,
): TimelineItem[] {
  const { t } = tr
  const items = new Map<string, TimelineItem>()
  const put = (item: TimelineItem) => items.set(item.key, item)

  const slotOfVariant = new Map<string, Slot>()
  const stepKindOf = new Map<string, StepKind>()
  const stepVersionOf = new Map<string, number>()
  for (const step of view?.steps ?? []) {
    stepKindOf.set(step.id, step.kind)
    stepVersionOf.set(step.id, step.version)
    for (const v of step.variants) {
      slotOfVariant.set(v.id, { language: v.language, channel: v.channel, timezone: v.timezone })
    }
  }

  const step = (kind: StepKind, version?: number) =>
    version !== undefined && version > 1
      ? t('step.withVersion', { step: labels.stepKind(kind), version })
      : labels.stepKind(kind)
  const target = (kind: StepKind, slot?: Slot) => {
    const slotLabel = slot ? labels.slot(slot) : ''
    return slotLabel ? `${labels.stepKind(kind)} · ${slotLabel}` : labels.stepKind(kind)
  }

  for (const entry of live) {
    switch (entry.name) {
      case 'screen_result': {
        const d = entry.data
        put({
          key: `screen:${d.at}`,
          at: d.at,
          kind: 'screen',
          author: 'system',
          tone: d.verdict === 'allow' ? 'success' : d.verdict === 'block' ? 'danger' : 'warning',
          title: t(`timeline.screen.${d.verdict}`),
          body: d.reason || undefined,
        })
        break
      }
      case 'step_opened': {
        const d = entry.data
        put({
          key: `step-opened:${d.step_id}`,
          at: d.at,
          kind: 'stepOpened',
          author: 'system',
          tone: 'neutral',
          title: t('timeline.stepOpened', { step: step(d.kind, d.version) }),
          stepId: d.step_id,
        })
        break
      }
      case 'step_approved': {
        const d = entry.data
        put({
          key: `step-approved:${d.step_id}`,
          at: d.at,
          kind: 'stepApproved',
          author: 'system',
          tone: 'success',
          title: t('timeline.stepApproved', { step: step(d.kind, d.version) }),
          stepId: d.step_id,
        })
        break
      }
      case 'step_superseded': {
        const d = entry.data
        put({
          key: `step-superseded:${d.step_id}`,
          at: d.at,
          kind: 'stepSuperseded',
          author: 'system',
          tone: 'warning',
          title: t('timeline.stepSuperseded', { step: step(d.kind, d.version) }),
        })
        break
      }
      case 'generation_started': {
        const d = entry.data
        put({
          key: `generation-started:${d.step_id}:${d.at}`,
          at: d.at,
          kind: 'generationStarted',
          author: 'system',
          tone: 'neutral',
          title: t('timeline.generationStarted', { step: labels.stepKind(d.kind) }),
          stepId: d.step_id,
        })
        break
      }
      case 'generation_failed': {
        const d = entry.data
        put({
          key: `generation-failed:${d.step_id}:${d.at}:${slotKey(d.slot)}`,
          at: d.at,
          kind: 'generationFailed',
          author: 'system',
          tone: 'danger',
          title: t('timeline.generationFailed', { target: target(d.kind, d.slot) }),
          body: d.reason,
          stepId: d.step_id,
        })
        break
      }
      case 'variant_created': {
        const d = entry.data
        put({
          key: `variant-created:${d.variant_id}`,
          at: d.at,
          kind: 'variantCreated',
          author: 'system',
          tone: 'neutral',
          title: t('timeline.variantCreated', { target: target(d.kind, d.slot) }),
          stepId: d.step_id,
        })
        break
      }
      case 'variant_approved': {
        const d = entry.data
        const kind = stepKindOf.get(d.step_id)
        put({
          key: `variant-approved:${d.variant_id}`,
          at: d.at,
          kind: 'variantApproved',
          author: 'system',
          tone: 'success',
          title: t('timeline.variantApproved', {
            target: kind ? target(kind, slotOfVariant.get(d.variant_id)) : t('timeline.variant'),
          }),
          stepId: d.step_id,
        })
        break
      }
      case 'variant_rejected': {
        const d = entry.data
        const kind = stepKindOf.get(d.step_id)
        put({
          key: `variant-rejected:${d.variant_id}`,
          at: d.at,
          kind: 'rejection',
          author: 'user',
          tone: 'neutral',
          title: t('timeline.rejection', {
            target: kind ? target(kind, slotOfVariant.get(d.variant_id)) : t('timeline.variant'),
          }),
          body: d.reason,
          stepId: d.step_id,
        })
        break
      }
      case 'context_added': {
        const d = entry.data
        put({
          key: `note:${d.note_id}`,
          at: d.at,
          kind: 'note',
          author: 'user',
          tone: 'neutral',
          title: t('timeline.note'),
          body: d.body,
          stepId: d.step_id,
        })
        break
      }
      case 'campaign_status': {
        const d = entry.data
        put({
          key: `status:${d.status}`,
          at: d.at,
          kind: 'campaignStatus',
          author: 'system',
          tone: d.status === 'completed' ? 'success' : 'neutral',
          title: t(`timeline.campaignStatus.${d.status}`),
        })
        break
      }
      case 'campaign_confirmed': {
        const d = entry.data
        put({
          key: 'campaign-confirmed',
          at: d.at,
          kind: 'campaignConfirmed',
          author: 'system',
          tone: 'success',
          title: t('timeline.campaignConfirmed', {
            total: d.recipient_counts.total ?? 0,
            email: d.recipient_counts.email ?? 0,
            sms: d.recipient_counts.sms ?? 0,
          }),
        })
        break
      }
      case 'campaign_cancelled': {
        const d = entry.data
        put({
          key: 'campaign-cancelled',
          at: d.at,
          kind: 'campaignCancelled',
          author: 'system',
          tone: 'danger',
          title: t('timeline.campaignCancelled'),
          body: d.reason || undefined,
        })
        break
      }
    }
  }

  // The view is written last so that, for a fact both know, the stored one wins.
  if (view) {
    put({
      key: 'brief',
      at: view.created_at,
      kind: 'brief',
      author: 'user',
      tone: 'neutral',
      title: t('timeline.brief'),
      body: view.brief,
    })
    for (const note of view.notes) {
      put({
        key: `note:${note.id}`,
        at: note.created_at,
        kind: 'note',
        author: 'user',
        tone: 'neutral',
        title: t('timeline.note'),
        body: note.body,
        stepId: note.step_id,
      })
    }
    for (const s of view.steps) {
      put({
        key: `step-opened:${s.id}`,
        at: s.created_at,
        kind: 'stepOpened',
        author: 'system',
        tone: 'neutral',
        title: t('timeline.stepOpened', { step: step(s.kind, s.version) }),
        stepId: s.id,
      })
      if (s.approved_at) {
        put({
          key: `step-approved:${s.id}`,
          at: s.approved_at,
          kind: 'stepApproved',
          author: 'system',
          tone: 'success',
          title: t('timeline.stepApproved', { step: step(s.kind, s.version) }),
          stepId: s.id,
        })
      }
      for (const v of s.variants) {
        const slot = { language: v.language, channel: v.channel, timezone: v.timezone }
        put({
          key: `variant-created:${v.id}`,
          at: v.created_at,
          kind: 'variantCreated',
          author: 'system',
          tone: 'neutral',
          title: t('timeline.variantCreated', { target: target(s.kind, slot) }),
          stepId: s.id,
        })
        if (v.approved_at) {
          put({
            key: `variant-approved:${v.id}`,
            at: v.approved_at,
            kind: 'variantApproved',
            author: 'system',
            tone: 'success',
            title: t('timeline.variantApproved', { target: target(s.kind, slot) }),
            stepId: s.id,
          })
        }
        if (v.rejected_at) {
          put({
            key: `variant-rejected:${v.id}`,
            at: v.rejected_at,
            kind: 'rejection',
            author: 'user',
            tone: 'neutral',
            title: t('timeline.rejection', { target: target(s.kind, slot) }),
            body: v.rejection_reason,
            stepId: s.id,
          })
        }
      }
    }
  }

  return [...items.values()].sort(
    (a, b) => instant(a.at) - instant(b.at) || RANK[a.kind] - RANK[b.kind],
  )
}
