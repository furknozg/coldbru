import { CHANNEL_COLORS, NEUTRAL_COLOR, SERIES_COLORS } from '@/features/campaigns/distribution'

import { OTHER_KEY, PURPOSES } from './series'

const escapeHtml = (text: string) => text.replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`)

/**
 * The tooltip of the dashboard charts, in the style of the pie's (D-092). Every piece of text goes
 * through escaping because model names and bucket labels come from the API.
 */
export function tooltipHtml(
  title: string,
  rows: readonly { label: string; color: string; value: string }[],
): string {
  const lines = rows
    .map(
      (row) =>
        `<div class="flex items-center gap-2">` +
        `<span class="size-2 shrink-0 rounded-full" style="background:${escapeHtml(row.color)}"></span>` +
        `<span class="text-muted-foreground">${escapeHtml(row.label)}</span>` +
        `<span class="ml-auto pl-3 font-medium tabular-nums">${escapeHtml(row.value)}</span></div>`,
    )
    .join('')
  return (
    `<div class="bg-popover text-popover-foreground flex min-w-32 flex-col gap-1 rounded-md border px-2 py-1.5 text-xs shadow-md">` +
    `<div class="font-medium">${escapeHtml(title)}</div>${lines}</div>`
  )
}

/** Channels keep their palette token wherever they are drawn (D-092). */
export const channelColor = (channel: string) => CHANNEL_COLORS[channel] ?? NEUTRAL_COLOR

/** A purpose is one entity, so it has one color in the pie, the area and the table. */
export const purposeColor = (purpose: string) => {
  const index = PURPOSES.indexOf(purpose as (typeof PURPOSES)[number])
  return index === -1 ? NEUTRAL_COLOR : SERIES_COLORS[index]!
}

/** Models take the palette in the order of their names, the summed tail is neutral. */
export const modelColors = (models: readonly string[]): Record<string, string> =>
  Object.fromEntries(
    models.map((model, index) => [
      model,
      model === OTHER_KEY ? NEUTRAL_COLOR : (SERIES_COLORS[index] ?? NEUTRAL_COLOR),
    ]),
  )

/**
 * Delivery statuses. Chart-1 and chart-2 belong to the channels, so no status reuses them; what
 * failed is the destructive red and what never ran is a muted neutral (D-092).
 */
export const STATUS_COLORS: Record<string, string> = {
  sent: 'var(--chart-3)',
  sending: 'var(--chart-4)',
  pending: NEUTRAL_COLOR,
  failed: 'var(--destructive)',
  skipped_no_consent: 'var(--chart-5)',
  cancelled: 'color-mix(in oklab, var(--muted-foreground) 75%, var(--background))',
}
export const DELIVERY_STATUSES = [
  'sent',
  'sending',
  'pending',
  'failed',
  'skipped_no_consent',
  'cancelled',
] as const
