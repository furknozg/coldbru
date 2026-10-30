import type { DeliveryCount, ModelUsage, PurposeUsage } from '@/api/analytics/service'
import { foldSmallest, type Slice } from '@/features/campaigns/distribution'

import { DELIVERY_STATUSES, channelColor, modelColors, purposeColor, STATUS_COLORS } from './chart'
import { CHANNEL_KEYS, MAX_MODELS, OTHER_KEY, PURPOSES, toNumber } from './series'

const sum = (counts: readonly DeliveryCount[], keep: (c: DeliveryCount) => boolean) =>
  counts.filter(keep).reduce((total, c) => total + c.count, 0)

/** Deliveries with this status over every channel. */
export const deliveriesWithStatus = (counts: readonly DeliveryCount[], status: string) =>
  sum(counts, (c) => c.status === status)

/** Deliveries per channel, over every status; both channels are always listed. */
export function channelSlices(
  counts: readonly DeliveryCount[],
  label: (channel: string) => string,
): Slice[] {
  return CHANNEL_KEYS.map((channel) => ({
    key: channel,
    label: label(channel),
    value: sum(counts, (c) => c.channel === channel),
    color: channelColor(channel),
  }))
}

/** Deliveries per status, over every channel; a status nobody is in is left out. */
export function statusSlices(
  counts: readonly DeliveryCount[],
  label: (status: string) => string,
): Slice[] {
  return DELIVERY_STATUSES.map((status) => ({
    key: status,
    label: label(status),
    value: deliveriesWithStatus(counts, status),
    color: STATUS_COLORS[status],
  })).filter((slice) => slice.value > 0)
}

/** LLM cost per purpose, in the order of the flow. */
export function purposeSlices(
  usage: readonly PurposeUsage[],
  label: (purpose: string) => string,
): Slice[] {
  return PURPOSES.map((purpose) => ({
    key: purpose,
    label: label(purpose),
    value: toNumber(usage.find((u) => u.purpose === purpose)?.cost_usd),
    color: purposeColor(purpose),
  })).filter((slice) => usage.some((u) => u.purpose === slice.key))
}

/** LLM cost per model; beyond the palette the smallest are summed into "other". */
export function modelSlices(usage: readonly ModelUsage[], other: string): Slice[] {
  const sorted = [...usage].sort((a, b) => a.model.localeCompare(b.model))
  const colors = modelColors(sorted.map((u) => u.model))
  const slices = sorted.map((u) => ({
    key: u.model,
    label: u.model,
    value: toNumber(u.cost_usd),
    color: colors[u.model],
  }))
  return foldSmallest(slices, MAX_MODELS - 1, { key: OTHER_KEY, label: other, value: 0 })
}

/** Input and output tokens together; cache read and creation tokens are not counted. */
export const totalTokens = (llm: { tokens: { input: number; output: number } }) =>
  llm.tokens.input + llm.tokens.output

/** Tokens served from or written to the prompt cache; they are priced apart from input and output. */
export const cacheTokens = (llm: { tokens: { cache_read: number; cache_creation: number } }) =>
  llm.tokens.cache_read + llm.tokens.cache_creation
