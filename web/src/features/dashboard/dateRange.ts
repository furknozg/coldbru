import type { SeriesQuery, TimeRange } from '@/api/analytics/service'

/**
 * The date filter of the dashboard (D-097). A range is two calendar dates in the browser's time
 * zone, both inclusive, written `YYYY-MM-DD` in the URL query (`?from=…&to=…`). No query means
 * "Tümü": every day Analytics knows. The API gets the half-open range [start of `from`, start of
 * the day after `to`) as RFC 3339 UTC.
 */
export interface DateRange {
  from: string | null
  to: string | null
}

export const ALL_TIME: DateRange = { from: null, to: null }

export const PRESETS = ['today', 'last7', 'last30', 'last90', 'all'] as const
export type Preset = (typeof PRESETS)[number]

const DAYS: Record<Exclude<Preset, 'all'>, number> = { today: 1, last7: 7, last30: 30, last90: 90 }

const DAY_MS = 24 * 60 * 60 * 1000
const HOUR_BUCKET_MAX_DAYS = 2
/** What "Tümü" (and an open-ended range) draws in a series: the last 90 days, by day. */
export const ALL_TIME_SERIES_DAYS = 90
/** The API allows at most 400 days of day buckets. */
const MAX_DAY_BUCKETS = 400

const DATE = /^(\d{4})-(\d{2})-(\d{2})$/

/** A local calendar date as `YYYY-MM-DD`. */
export function formatDate(date: Date): string {
  const pad = (n: number, width = 2) => String(n).padStart(width, '0')
  return `${pad(date.getFullYear(), 4)}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

/** Local midnight of a `YYYY-MM-DD` date, or `null` when it is no real date. */
export function parseDate(text: string): Date | null {
  const match = DATE.exec(text)
  if (!match) return null
  const [year, month, day] = [Number(match[1]), Number(match[2]) - 1, Number(match[3])]
  const date = new Date(year, month, day)
  const real = date.getFullYear() === year && date.getMonth() === month && date.getDate() === day
  return real ? date : null
}

const addDays = (date: Date, days: number) =>
  new Date(date.getFullYear(), date.getMonth(), date.getDate() + days)

const startOfDay = (date: Date) => new Date(date.getFullYear(), date.getMonth(), date.getDate())

/** RFC 3339 in UTC without fractions: `2026-10-29T21:00:00Z`. */
export function toRfc3339(date: Date): string {
  return date.toISOString().replace(/\.\d{3}Z$/, 'Z')
}

/** The range a preset stands for on the day of `now`; "Tümü" is the empty range. */
export function presetRange(preset: Preset, now: Date): DateRange {
  if (preset === 'all') return ALL_TIME
  const today = startOfDay(now)
  return { from: formatDate(addDays(today, 1 - DAYS[preset])), to: formatDate(today) }
}

/** The preset a range equals today, or `null` for a custom range. */
export function activePreset(range: DateRange, now: Date): Preset | null {
  return PRESETS.find((p) => sameRange(presetRange(p, now), range)) ?? null
}

const sameRange = (a: DateRange, b: DateRange) => a.from === b.from && a.to === b.to

/**
 * Reads the range from a route query. A bound that is not a real date is dropped, and a range that
 * runs backwards is swapped, so a hand-edited URL never reaches the API as a 422.
 */
export function rangeFromQuery(query: Record<string, unknown>): DateRange {
  const read = (value: unknown) => {
    const text = Array.isArray(value) ? value[0] : value
    return typeof text === 'string' && parseDate(text) ? text : null
  }
  const from = read(query.from)
  const to = read(query.to)
  if (from && to && from > to) return { from: to, to: from }
  return { from, to }
}

/** The query that carries a range; "Tümü" has none. */
export function rangeToQuery(range: DateRange): { from?: string; to?: string } {
  return { ...(range.from ? { from: range.from } : {}), ...(range.to ? { to: range.to } : {}) }
}

/** The range as the totals endpoints take it: an open bound stays out of the request. */
export function toApiRange(range: DateRange): TimeRange {
  const out: TimeRange = {}
  const from = range.from && parseDate(range.from)
  const to = range.to && parseDate(range.to)
  if (from) out.from = toRfc3339(from)
  if (to) out.to = toRfc3339(addDays(to, 1))
  return out
}

/**
 * The window and bucket size of a series chart. Both ends are needed, so an open end is filled in:
 * the end is the start of tomorrow, the start is 90 days before the end. Up to two days are drawn
 * by hour, longer ranges by day; more than 400 days keeps the latest 400 (the API's limit).
 */
export function seriesQuery(range: DateRange, now: Date): SeriesQuery {
  const from = range.from ? parseDate(range.from) : null
  const to = range.to ? parseDate(range.to) : null
  const end = to ? addDays(to, 1) : addDays(startOfDay(now), 1)
  let start = from ?? addDays(end, -ALL_TIME_SERIES_DAYS)
  if (start >= end) start = addDays(end, -1)
  // Calendar days between the two local midnights; a DST change shifts the diff by an hour.
  const days = Math.round((end.getTime() - start.getTime()) / DAY_MS)
  if (days > MAX_DAY_BUCKETS) start = addDays(end, -MAX_DAY_BUCKETS)
  return {
    from: toRfc3339(start),
    to: toRfc3339(end),
    bucket: days <= HOUR_BUCKET_MAX_DAYS ? 'hour' : 'day',
  }
}
