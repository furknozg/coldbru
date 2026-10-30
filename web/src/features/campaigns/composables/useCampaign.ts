import { computed, onScopeDispose, shallowRef, toValue, watch, type MaybeRefOrGetter } from 'vue'

import { ApiError } from '@/api/errors'
import { proposalEventsUrl, type ProposalEvents } from '@/api/events'
import { getCampaign } from '@/api/proposal/service'
import { useEventStream, type EventHandlers } from '@/api/useEventStream'

import { TIMELINE_EVENTS, type LiveEntry, type TimelineEventName } from '../timeline'
import type { CampaignView, Slot } from '../types'
import { slotKey } from '../types'

/** A slot whose text the model is writing right now (`content_delta`, D-072). */
export interface StreamingSlot {
  key: string
  stepId: string
  slot: Slot
  attempt: number
  text: string
}

/** Events arriving together (a content step opens several variants) cause one refetch. */
const REFRESH_DELAY_MS = 150
/** The newest events kept for the timeline; older ones are in the view anyway. */
const MAX_LIVE = 200

const streamKey = (stepId: string, slot: Slot) => `${stepId}|${slotKey(slot)}`

/**
 * One campaign: its view, the events received since the page opened and the text being written.
 *
 * The view comes from `GET /campaigns/{id}` and from the `snapshot` the stream sends first on every
 * connection (D-072); every other state-changing event schedules a refetch, so the view always
 * has the shape the server gives it. Opening another id drops the old state and, through
 * `useEventStream`, closes the old connection first: one stream at a time.
 */
export function useCampaign(campaignId: MaybeRefOrGetter<string | null>) {
  const view = shallowRef<CampaignView | null>(null)
  const loading = shallowRef(false)
  const error = shallowRef<unknown>(null)
  const live = shallowRef<readonly LiveEntry[]>([])
  const streams = shallowRef<Readonly<Record<string, StreamingSlot>>>({})

  /** Bumped by every response that is applied, so an older answer cannot overwrite a newer one. */
  let generation = 0
  let timer: ReturnType<typeof setTimeout> | undefined

  const id = () => toValue(campaignId)

  async function refresh() {
    const target = id()
    if (!target) return
    const mine = ++generation
    loading.value = true
    try {
      const next = await getCampaign(target)
      if (mine !== generation || target !== id()) return
      view.value = next
      error.value = null
    } catch (cause) {
      if (mine !== generation || target !== id()) return
      // A failed refetch keeps what is on screen; only the first load is an error state.
      if (view.value === null) error.value = cause
    } finally {
      if (mine === generation) loading.value = false
    }
  }

  function scheduleRefresh() {
    clearTimeout(timer)
    timer = setTimeout(() => void refresh(), REFRESH_DELAY_MS)
  }

  /** Applies a view a mutation returned, as if a snapshot had arrived. */
  function applyView(next: CampaignView) {
    generation++
    view.value = next
    error.value = null
  }

  function record<K extends TimelineEventName>(name: K, data: ProposalEvents[K]) {
    const entry = { name, data } as LiveEntry
    live.value = [...live.value, entry].slice(-MAX_LIVE)
  }

  function dropStreams(match: (s: StreamingSlot) => boolean) {
    const next = Object.fromEntries(Object.entries(streams.value).filter(([, s]) => !match(s)))
    if (Object.keys(next).length !== Object.keys(streams.value).length) streams.value = next
  }

  const handlers = {
    snapshot(data: CampaignView) {
      applyView(data)
    },
    content_delta(data: ProposalEvents['content_delta']) {
      const key = streamKey(data.step_id, data.slot)
      const current = streams.value[key]
      const text =
        current && current.attempt === data.attempt ? current.text + data.text : data.text
      streams.value = {
        ...streams.value,
        [key]: { key, stepId: data.step_id, slot: data.slot, attempt: data.attempt, text },
      }
    },
    content_reset(data: ProposalEvents['content_reset']) {
      const key = streamKey(data.step_id, data.slot)
      streams.value = {
        ...streams.value,
        [key]: { key, stepId: data.step_id, slot: data.slot, attempt: data.attempt, text: '' },
      }
    },
  }

  // One handler per timeline event: remember it, keep the stream bookkeeping right, refetch.
  const eventHandlers = Object.fromEntries(
    TIMELINE_EVENTS.map((name) => [
      name,
      (data: ProposalEvents[typeof name]) => {
        record(name, data as never)
        if (name === 'variant_created') {
          const d = data as ProposalEvents['variant_created']
          dropStreams((s) => s.key === streamKey(d.step_id, d.slot))
        } else if (name === 'generation_failed') {
          const d = data as ProposalEvents['generation_failed']
          const hasSlot = Boolean(d.slot.language || d.slot.channel || d.slot.timezone)
          dropStreams(
            (s) => s.stepId === d.step_id && (!hasSlot || s.key === streamKey(d.step_id, d.slot)),
          )
        } else if (name === 'step_superseded') {
          const d = data as ProposalEvents['step_superseded']
          dropStreams((s) => s.stepId === d.step_id)
        }
        if (name !== 'screen_result') scheduleRefresh()
      },
    ]),
  )

  const url = computed(() => {
    const target = id()
    return target ? proposalEventsUrl(target) : null
  })

  const stream = useEventStream<ProposalEvents>(url, {
    ...(eventHandlers as EventHandlers<ProposalEvents>),
    ...handlers,
  })

  watch(
    id,
    (target) => {
      generation++
      clearTimeout(timer)
      view.value = null
      error.value = null
      live.value = []
      streams.value = {}
      loading.value = false
      if (target) void refresh()
    },
    { immediate: true },
  )

  onScopeDispose(() => clearTimeout(timer))

  return {
    view: computed(() => view.value),
    /** True while a request for the view is running. */
    loading: computed(() => loading.value),
    /** Why the first load failed; `null` once a view is there. */
    error: computed(() => error.value),
    notFound: computed(() => error.value instanceof ApiError && error.value.status === 404),
    live,
    /** Slots being written, in the order they started. */
    streaming: computed(() => Object.values(streams.value)),
    connection: stream.status,
    refresh,
    applyView,
  }
}

export type CampaignState = ReturnType<typeof useCampaign>
