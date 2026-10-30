import { shallowRef, toValue, watch, type MaybeRefOrGetter } from 'vue'

import type { components } from '@/api/notification/schema'
import { notificationEventsUrl, type NotificationEvents } from '@/api/events'
import { useEventStream } from '@/api/useEventStream'

export type DeliverySummary = components['schemas']['api.DeliverySummary']
export type DoneStatus = components['schemas']['api.DoneEvent']['status']

/**
 * The live delivery counts of one campaign from Notification's SSE (D-080). The server sends the
 * current counts when the connection opens, a new snapshot whenever they change, and `done` when
 * the campaign ends; after `done` the stream is closed, or the browser would reconnect to a
 * finished campaign. Switching campaigns clears the counts and opens the next stream; leaving the
 * page (scope disposal) closes it.
 */
export function useLiveDeliveries(campaignId: MaybeRefOrGetter<string | null>) {
  const summary = shallowRef<DeliverySummary | null>(null)
  const done = shallowRef<DoneStatus | null>(null)

  // Declared before the stream so it runs first when the campaign changes.
  watch(
    () => toValue(campaignId),
    () => {
      summary.value = null
      done.value = null
    },
    { flush: 'sync' },
  )

  const stream = useEventStream<NotificationEvents>(
    () => {
      const id = toValue(campaignId)
      return id ? notificationEventsUrl(id) : null
    },
    {
      snapshot: (snapshot) => {
        summary.value = snapshot
      },
      done: (event) => {
        done.value = event.status
        stream.close()
      },
    },
  )

  return { summary, done, status: stream.status }
}
