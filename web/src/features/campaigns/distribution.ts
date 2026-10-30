import type { AudienceContent, Coverage, LocaleAudience } from './content'

/** One slice of a distribution chart; `color` is any CSS color (usually a palette token). */
export interface Slice {
  key: string
  label: string
  value: number
  color?: string
}

/**
 * Colors follow the entity, not its rank (D-092): the channels keep their palette token in every
 * chart, a recipient set that mixes both channels is espresso, and what cannot be reached is a
 * muted neutral derived from the same tokens so it also follows the dark theme.
 */
export const NEUTRAL_COLOR = 'color-mix(in oklab, var(--muted-foreground) 45%, var(--background))'
export const CHANNEL_COLORS: Record<string, string> = {
  email: 'var(--chart-1)',
  sms: 'var(--chart-2)',
  email_only: 'var(--chart-1)',
  sms_only: 'var(--chart-2)',
  both: 'var(--chart-3)',
  unreachable: NEUTRAL_COLOR,
}

/** The colors categories without a fixed meaning (languages, locales) take, in this order. */
export const SERIES_COLORS = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
  'var(--chart-5)',
] as const

export const COVERAGE_KINDS = ['both', 'email_only', 'sms_only', 'unreachable'] as const

export function coverageSlices(
  coverage: Coverage,
  label: (kind: (typeof COVERAGE_KINDS)[number]) => string,
): Slice[] {
  return COVERAGE_KINDS.map((kind) => ({ key: kind, label: label(kind), value: coverage[kind] }))
}

/**
 * People per language. A variant from D-076 on carries `by_locale` (language + time zone), whose
 * rows are summed per language; an older one only has `by_language`.
 */
export function languageTotals(audience: AudienceContent): { language: string; count: number }[] {
  const rows: LocaleAudience[] = audience.byLocale.length ? audience.byLocale : audience.byLanguage
  const totals = new Map<string, number>()
  for (const row of rows) totals.set(row.language, (totals.get(row.language) ?? 0) + row.count)
  return [...totals].map(([language, count]) => ({ language, count }))
}

/** Largest first, ties by key, so the colors do not move between two renders of the same data. */
export function bySize<T extends { key: string; value: number }>(slices: T[]): T[] {
  return [...slices].sort((a, b) => b.value - a.value || a.key.localeCompare(b.key))
}

/**
 * Keeps the `max` largest slices and sums the rest into one "other" slice: a pie with more
 * categories than the palette has hues stops being readable (D-092).
 */
export function foldSmallest(slices: Slice[], max: number, other: Slice): Slice[] {
  if (slices.length <= max) return slices
  const sorted = bySize(slices)
  const rest = sorted.slice(max)
  return [
    ...sorted.slice(0, max),
    { ...other, value: rest.reduce((sum, slice) => sum + slice.value, 0), color: NEUTRAL_COLOR },
  ]
}
