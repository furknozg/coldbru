import type { LlmSeries, OverviewSeries } from '@/api/analytics/service'

/** One line of a time series chart: a value for every bucket, in the order of `buckets`. */
export interface SeriesLine {
  key: string
  values: number[]
}

/** Channels in the fixed order the charts and their colors use (D-092). */
export const CHANNEL_KEYS = ['email', 'sms'] as const

/** LLM purposes in the order of the flow (D-033); a purpose keeps its color in every chart. */
export const PURPOSES = ['screen', 'segment', 'channel_plan', 'content', 'schedule'] as const

/** Key of the line that sums the models beyond the palette. */
export const OTHER_KEY = 'other'
/** Models drawn before the rest is summed into "other": the palette has five hues (D-092). */
export const MAX_MODELS = 5

/**
 * The series endpoints send sparse rows and every bucket start (D-097). This lays the rows on the
 * buckets and fills the missing cells with 0, one line per key. Rows are matched to buckets by
 * instant, not by text, and rows outside the buckets or of an unknown key are ignored.
 */
export function pivot<P extends { start: string }>(
  buckets: readonly string[],
  points: readonly P[],
  keys: readonly string[],
  keyOf: (point: P) => string,
  valueOf: (point: P) => number,
): SeriesLine[] {
  const at = new Map(buckets.map((start, index) => [Date.parse(start), index]))
  const lines = new Map(keys.map((key) => [key, Array.from({ length: buckets.length }, () => 0)]))
  for (const point of points) {
    const values = lines.get(keyOf(point))
    const index = at.get(Date.parse(point.start))
    if (values && index !== undefined) values[index]! += valueOf(point)
  }
  return keys.map((key) => ({ key, values: lines.get(key)! }))
}

/** Deliveries with the given status per bucket, one line per channel (D-097). */
export function deliveryLines(series: OverviewSeries, status = 'sent'): SeriesLine[] {
  return pivot(
    series.buckets,
    series.points.filter((p) => p.status === status),
    CHANNEL_KEYS,
    (p) => p.channel,
    (p) => p.count,
  )
}

/** Cost is a decimal string in dollars; anything that is not a number counts as 0. */
export const toNumber = (text: string | number | null | undefined) => {
  const value = Number(text)
  return Number.isFinite(value) ? value : 0
}

/** Model names in a stable order, the biggest spenders kept and the rest summed into "other". */
export function modelKeys(series: LlmSeries): string[] {
  const cost = new Map<string, number>()
  for (const p of series.points) cost.set(p.model, (cost.get(p.model) ?? 0) + toNumber(p.cost_usd))
  const models = [...cost.keys()]
  if (models.length <= MAX_MODELS) return models.sort()
  const kept = models
    .sort((a, b) => cost.get(b)! - cost.get(a)! || a.localeCompare(b))
    .slice(0, MAX_MODELS - 1)
    .sort()
  return [...kept, OTHER_KEY]
}

/** Cost per bucket stacked by purpose or by model. */
export function llmCostLines(series: LlmSeries, by: 'purpose' | 'model'): SeriesLine[] {
  if (by === 'purpose') {
    return pivot(
      series.buckets,
      series.points,
      PURPOSES,
      (p) => p.purpose,
      (p) => toNumber(p.cost_usd),
    )
  }
  const keys = modelKeys(series)
  return pivot(
    series.buckets,
    series.points,
    keys,
    (p) => (keys.includes(p.model) ? p.model : OTHER_KEY),
    (p) => toNumber(p.cost_usd),
  )
}

/** All LLM calls per bucket. */
export function llmCallsLine(series: LlmSeries): SeriesLine {
  return pivot(
    series.buckets,
    series.points,
    ['calls'],
    () => 'calls',
    (p) => p.calls,
  )[0]!
}

export const sumOf = (line: SeriesLine) => line.values.reduce((sum, v) => sum + v, 0)
