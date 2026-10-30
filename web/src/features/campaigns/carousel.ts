import { slotKey, type Channel, type StepView, type VariantView } from './types'

/**
 * How many slots (language + time zone + channel) of a step still wait for an approved variant
 * (D-095). A slot counts once, however many drafts or rejected tries it has; a slot the step has
 * not produced anything for yet (`missing`) counts too. With `channel`, only that channel's slots.
 */
export function pendingSlotCount(step: StepView, channel?: Channel): number {
  const inScope = (slot: { channel?: Channel }) => !channel || slot.channel === channel
  const approved = new Set<string>()
  const all = new Set<string>()
  for (const variant of step.variants.filter(inScope)) {
    const key = slotKey(variant)
    all.add(key)
    if (variant.status === 'approved') approved.add(key)
  }
  for (const slot of (step.missing ?? []).filter(inScope)) all.add(slotKey(slot))
  let pending = 0
  for (const key of all) if (!approved.has(key)) pending++
  return pending
}

/**
 * The slide to show after the one at `from` was approved: the next draft, wrapping around, whose
 * slot has no approved variant. -1 when every slot is approved (or the only drafts left share a
 * slot with an approved one).
 */
export function nextPendingIndex(slides: readonly VariantView[], from: number): number {
  const approved = new Set(slides.filter((v) => v.status === 'approved').map(slotKey))
  for (let step = 1; step <= slides.length; step++) {
    const index = (from + step) % slides.length
    const slide = slides[index]!
    if (slide.status === 'draft' && !approved.has(slotKey(slide))) return index
  }
  return -1
}

/**
 * The order of the slides of one carousel: language, then time zone. Variants of one slot (several
 * drafts, an approved one) keep the order the step lists them in, so a slide never jumps when
 * another variant arrives for a different slot.
 */
export function orderSlides(variants: readonly VariantView[]): VariantView[] {
  return variants
    .map((variant, listed) => ({ variant, listed }))
    .sort(
      (a, b) =>
        (a.variant.language ?? '').localeCompare(b.variant.language ?? '') ||
        (a.variant.timezone ?? '').localeCompare(b.variant.timezone ?? '') ||
        a.listed - b.listed,
    )
    .map((entry) => entry.variant)
}

/** One carousel of a step: its variants and the slots among them that still wait. */
export interface CarouselGroup {
  key: string
  /** Set for the per-channel carousels of the text step; the label is the channel's name. */
  channel?: Channel
  variants: VariantView[]
  pending: number
}

const CHANNELS: readonly Channel[] = ['email', 'sms']

/**
 * The carousels of a step's cards (D-095): the text step has one per channel, e-mail first; the
 * schedule step has one for all its slots. A group shows when it has a card or still has slots
 * waiting (all rejected, or not produced yet), which it then tells in place of slides; a group
 * without any slot stays out. `open` is false once the step is approved, when nothing waits.
 */
export function carouselGroups(
  step: StepView,
  cards: readonly VariantView[],
  open: boolean,
): CarouselGroup[] {
  const group = (
    key: string,
    variants: VariantView[],
    pending: number,
    channel?: Channel,
  ): CarouselGroup[] =>
    variants.length || pending > 0 ? [{ key, channel, variants, pending }] : []

  if (step.kind === 'content') {
    return CHANNELS.flatMap((channel) =>
      group(
        channel,
        cards.filter((v) => v.channel === channel),
        open ? pendingSlotCount(step, channel) : 0,
        channel,
      ),
    )
  }
  return group(step.kind, [...cards], open ? pendingSlotCount(step) : 0)
}
