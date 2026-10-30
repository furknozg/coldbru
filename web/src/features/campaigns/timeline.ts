import type { ProposalEvents } from '@/api/events'

import type { Labels, Translator } from './labels'
import type { CampaignView, ChatMessageView, Slot, StepKind } from './types'
import { slotKey } from './types'

/**
 * Events that become chat items. The rest (ping, snapshot, content stream, campaign_titled, which
 * only changes the title) never do.
 */
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
  'question_asked',
  'answer_given',
  'summary_posted',
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
  | 'question'
  | 'answer'
  | 'generationFailed'
  | 'variantCreated'
  | 'summary'
  | 'variantApproved'
  | 'stepApproved'
  | 'stepSuperseded'
  | 'campaignConfirmed'
  | 'campaignStatus'
  | 'campaignCancelled'

export interface TimelineItem {
  /** Same fact, same key: a live event and the view that now contains it are one item. */
  key: string
  at: string
  kind: TimelineKind
  /** `user` items are what the marketer wrote. */
  author: 'user' | 'system'
  tone: 'neutral' | 'success' | 'warning' | 'danger'
  title: string
  /**
   * The text of the item. Plain text with its line breaks, except when `markdown` is set: then it is
   * the agent's markdown (a question, a summary, D-088), rendered through `renderMarkdown`.
   */
  body?: string
  markdown?: boolean
  /** A question of the agent: its message id, for the answer, and its suggested answers. */
  question?: { id: string; options: readonly string[] }
  /** The step the item is about; the chat files the step's events under that step (D-091). */
  stepId?: string
}

/**
 * Tie-break for items of one step with the same instant: the order things happen in. Items of
 * different steps at the same instant (an approval that opens the next step in one transaction)
 * are ordered by their step first; see `sequenceOf`.
 */
const RANK: Record<TimelineKind, number> = {
  brief: 0,
  note: 1,
  screen: 2,
  stepOpened: 3,
  generationStarted: 4,
  question: 5,
  answer: 6,
  generationFailed: 7,
  variantCreated: 8,
  summary: 9,
  rejection: 10,
  variantApproved: 11,
  stepApproved: 12,
  stepSuperseded: 13,
  campaignConfirmed: 14,
  campaignStatus: 15,
  campaignCancelled: 16,
}

export const rankOf = (kind: TimelineKind) => RANK[kind]

export const instantOf = (at: string) => {
  const ms = Date.parse(at)
  return Number.isNaN(ms) ? 0 : ms
}

/**
 * Where an item belongs among items of the same instant: campaign-level items that start the
 * story first, then the steps in flow order (a step the view does not know yet last among them),
 * then the campaign-level items that end it.
 */
export function sequenceOf(item: TimelineItem, stepIndexOf: ReadonlyMap<string, number>): number {
  if (item.stepId) return stepIndexOf.get(item.stepId) ?? Number.MAX_SAFE_INTEGER - 1
  return RANK[item.kind] <= RANK.screen ? -1 : Number.MAX_SAFE_INTEGER
}

/**
 * The items of the chat (D-087, D-091): the marketer's messages, the agent's questions and
 * summaries, and system events, oldest first. History comes from the view, so a campaign opened
 * later still tells its story; what only an event knows (screening, generation start and failure,
 * supersession, sending) comes from `live`. The steps themselves are not items: `buildChat` draws
 * each step as one accordion that holds the step's events.
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
  const stepIndexOf = new Map<string, number>()
  for (const [index, step] of (view?.steps ?? []).entries()) {
    stepKindOf.set(step.id, step.kind)
    stepVersionOf.set(step.id, step.version)
    stepIndexOf.set(step.id, index)
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

  /** A chat message of the agent or the marketer (D-088); `slot` names a content slot. */
  const message = (m: ChatMessageView, slot?: Slot): TimelineItem | null => {
    const kind = m.step_id ? stepKindOf.get(m.step_id) : undefined
    const stepLabel = kind ? labels.stepKind(kind) : t('timeline.variant')
    switch (m.kind) {
      case 'question':
        return {
          key: `message:${m.id}`,
          at: m.created_at,
          kind: 'question',
          author: 'system',
          tone: 'warning',
          title: t('timeline.question', { step: stepLabel }),
          body: m.body,
          markdown: true,
          question: { id: m.id, options: m.options ?? [] },
          stepId: m.step_id,
        }
      case 'answer':
        return {
          key: `message:${m.id}`,
          at: m.created_at,
          kind: 'answer',
          author: 'user',
          tone: 'neutral',
          title: t('timeline.answer'),
          body: m.body,
          stepId: m.step_id,
        }
      case 'summary':
        return {
          key: `message:${m.id}`,
          at: m.created_at,
          kind: 'summary',
          author: 'system',
          tone: 'neutral',
          title: t('timeline.summary', { target: kind ? target(kind, slot) : stepLabel }),
          body: m.body,
          markdown: true,
          stepId: m.step_id,
        }
    }
    return null
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
          stepId: d.step_id,
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
      case 'question_asked': {
        const d = entry.data
        const item = message({
          id: d.message_id,
          campaign_id: d.campaign_id,
          step_id: d.step_id,
          kind: 'question',
          body: d.question,
          options: d.options,
          created_at: d.at,
        })
        if (item) put(item)
        break
      }
      case 'answer_given': {
        const d = entry.data
        const item = message({
          id: d.message_id,
          campaign_id: d.campaign_id,
          step_id: d.step_id,
          kind: 'answer',
          body: d.answer,
          reply_to: d.question_id,
          created_at: d.at,
        })
        if (item) put(item)
        break
      }
      case 'summary_posted': {
        const d = entry.data
        const item = message(
          {
            id: d.message_id,
            campaign_id: d.campaign_id,
            step_id: d.step_id,
            kind: 'summary',
            body: d.markdown,
            created_at: d.at,
          },
          d.slot,
        )
        if (item) put(item)
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
    for (const m of view.messages ?? []) {
      const item = message(m)
      if (item) put(item)
    }
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
      // An approved step always has approved_at; created_at covers a row from an older build.
      if (s.status === 'approved' || s.approved_at) {
        put({
          key: `step-approved:${s.id}`,
          at: s.approved_at ?? s.created_at,
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
    (a, b) =>
      instantOf(a.at) - instantOf(b.at) ||
      sequenceOf(a, stepIndexOf) - sequenceOf(b, stepIndexOf) ||
      RANK[a.kind] - RANK[b.kind],
  )
}
