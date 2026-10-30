import type { DeliverySummary } from './useLiveDeliveries'

/** Delivery statuses in the order of a delivery's life; the live table has a column for each. */
export const LIVE_STATUSES = [
  'pending',
  'sending',
  'sent',
  'failed',
  'skipped_no_consent',
  'cancelled',
] as const

/** A delivery in one of these will not change again. */
const FINISHED = new Set(['sent', 'failed', 'skipped_no_consent', 'cancelled'])

export const LIVE_CHANNELS = ['email', 'sms'] as const

/** Count of a channel and status; a pair the server leaves out has none. */
export function countOf(summary: DeliverySummary, channel: string, status: string): number {
  return summary.counts.find((c) => c.channel === channel && c.status === status)?.count ?? 0
}

/** How many deliveries have reached an end state, and the share of the total (0 to 1). */
export function progressOf(summary: DeliverySummary): { finished: number; ratio: number } {
  const finished = summary.counts
    .filter((c) => FINISHED.has(c.status))
    .reduce((total, c) => total + c.count, 0)
  return { finished, ratio: summary.total > 0 ? Math.min(finished / summary.total, 1) : 0 }
}

/** Statuses "Canlı gönderim" lists, in the order of its groups: being sent, then waiting to be sent. */
export const LIVE_CAMPAIGN_STATUSES = ['sending', 'scheduled'] as const

/** Statuses Kampanya analitiği lists (D-100): a campaign whose sending has started or ended. */
export const ANALYTICS_STATUSES = ['sending', 'completed'] as const
