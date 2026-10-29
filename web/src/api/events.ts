import { serviceBase } from './client'
import type { components as NotificationComponents } from './notification/schema'
import type { components as ProposalComponents } from './proposal/schema'

/**
 * SSE event name to JSON payload, taken straight from the `api.EventCatalog` schema each service
 * publishes next to its stream (D-072, D-080). Regenerating the schema changes these types, so
 * `useEventStream` handlers stop compiling when an event is renamed or reshaped.
 */
export type ProposalEvents = ProposalComponents['schemas']['api.EventCatalog']
export type NotificationEvents = NotificationComponents['schemas']['api.EventCatalog']

/** SSE URL of a campaign's flow events (snapshot first, D-072). */
export const proposalEventsUrl = (campaignId: string) =>
  `${serviceBase('proposal')}/campaigns/${encodeURIComponent(campaignId)}/events`

/** SSE URL of a campaign's live delivery counts (snapshot first, ends with `done`, D-080). */
export const notificationEventsUrl = (campaignId: string) =>
  `${serviceBase('notification')}/campaigns/${encodeURIComponent(campaignId)}/deliveries/events`
