import { instantOf, rankOf, sequenceOf, type TimelineItem, type TimelineKind } from './timeline'
import type { CampaignView, StepView } from './types'

/**
 * What the chat is made of (D-091): plain lines (the marketer's messages, the agent's questions and
 * summaries, campaign-level events) and one block per step. The block holds the step's events and
 * actions in one accordion and, as the only cards of the chat, its variants.
 */
export type ChatEntry =
  | { type: 'line'; key: string; item: TimelineItem }
  | { type: 'step'; key: string; step: StepView; events: readonly TimelineItem[] }

/** Items that tell what happened to a step; they live in the step's accordion, not in the chat. */
const STEP_EVENTS: ReadonlySet<TimelineKind> = new Set([
  'stepOpened',
  'generationStarted',
  'generationFailed',
  'variantCreated',
  'variantApproved',
  'stepApproved',
  'stepSuperseded',
])

/** Lines in the voice of a person or the agent; the rest of the lines are system events. */
const MESSAGES: ReadonlySet<TimelineKind> = new Set([
  'brief',
  'note',
  'rejection',
  'question',
  'answer',
  'summary',
])

export const isMessage = (item: TimelineItem) => MESSAGES.has(item.kind)

/**
 * Orders the timeline into chat entries. A step that is over (approved, replaced) stands where it
 * was approved; the open step stands last, after every line, with its variants under it. An event
 * of a step the view does not know (a stream ahead of its refetch) stays a line of its own.
 */
export function buildChat(view: CampaignView, items: readonly TimelineItem[]): ChatEntry[] {
  const stepIndexOf = new Map(view.steps.map((s, index) => [s.id, index] as const))
  const eventsOf = new Map<string, TimelineItem[]>()
  const entries: { entry: ChatEntry; order: Order }[] = []

  for (const item of items) {
    if (item.stepId && stepIndexOf.has(item.stepId) && STEP_EVENTS.has(item.kind)) {
      const list = eventsOf.get(item.stepId) ?? []
      list.push(item)
      eventsOf.set(item.stepId, list)
    } else {
      entries.push({
        entry: { type: 'line', key: item.key, item },
        order: {
          at: instantOf(item.at),
          sequence: sequenceOf(item, stepIndexOf),
          rank: rankOf(item.kind),
        },
      })
    }
  }

  const stepEntry = (step: StepView): ChatEntry => ({
    type: 'step',
    key: `step:${step.id}`,
    step,
    events: eventsOf.get(step.id) ?? [],
  })

  const open = view.steps.find((s) => s.status === 'open')
  for (const step of view.steps) {
    if (step === open) continue
    entries.push({
      entry: stepEntry(step),
      order: {
        at: instantOf(step.approved_at ?? step.created_at),
        sequence: stepIndexOf.get(step.id)!,
        rank: rankOf('stepApproved'),
      },
    })
  }
  entries.sort(
    (a, b) =>
      a.order.at - b.order.at || a.order.sequence - b.order.sequence || a.order.rank - b.order.rank,
  )

  const sorted = entries.map((e) => e.entry)
  if (open) sorted.push(stepEntry(open))
  return sorted
}

interface Order {
  at: number
  sequence: number
  rank: number
}
