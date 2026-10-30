<script setup lang="ts">
import { Donut } from '@unovis/ts'
import { VisDonut, VisSingleContainer } from '@unovis/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { ChartContainer, ChartTooltip, type ChartConfig } from '@/components/ui/chart'
import { NEUTRAL_COLOR, SERIES_COLORS, type Slice } from '@/features/campaigns/distribution'

/**
 * A part-to-whole chart: a donut for the shape and a legend of real text for the numbers (D-092).
 * Slices with no value stay in the legend but get no arc; with nothing at all there is no chart.
 * No card, border or box of its own: the variant card around it is the only surface (D-091).
 */
const props = defineProps<{ title: string; slices: Slice[] }>()

const { t, n } = useI18n()

const total = computed(() => props.slices.reduce((sum, slice) => sum + slice.value, 0))

const entries = computed(() =>
  props.slices.map((slice, index) => ({
    ...slice,
    // Styled through the `--color-<id>` variables the shadcn container defines from the config.
    id: `s${index}`,
    color: slice.color ?? SERIES_COLORS[index] ?? NEUTRAL_COLOR,
    percent: total.value > 0 ? slice.value / total.value : 0,
  })),
)
const arcs = computed(() => entries.value.filter((entry) => entry.value > 0))

const config = computed<ChartConfig>(() =>
  Object.fromEntries(entries.value.map((e) => [e.id, { label: e.label, color: e.color }])),
)

const summary = computed(() =>
  t('chart.summary', {
    title: props.title,
    items: entries.value
      .map((e) => `${e.label}: ${n(e.value, 'int')} (${n(e.percent, 'percent')})`)
      .join(', '),
  }),
)

const escapeHtml = (text: string) => text.replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`)

type Arc = { data: (typeof entries.value)[number] }
// Label text can come from the API (a time zone), so it is escaped before it becomes tooltip HTML.
const triggers = {
  [Donut.selectors.segment]: ({ data }: Arc) =>
    `<div class="bg-popover text-popover-foreground rounded-md border px-2 py-1 text-xs shadow-md">` +
    `${escapeHtml(data.label)}: <span class="font-medium tabular-nums">${escapeHtml(
      n(data.value, 'int'),
    )}</span> (${escapeHtml(n(data.percent, 'percent'))})</div>`,
}
</script>

<template>
  <figure class="flex min-w-0 flex-col gap-2">
    <figcaption class="text-muted-foreground text-xs font-medium">{{ title }}</figcaption>
    <p v-if="total === 0" class="text-muted-foreground text-xs">{{ t('chart.noData') }}</p>
    <div v-else class="flex flex-wrap items-center gap-x-4 gap-y-2">
      <ChartContainer
        :config="config"
        role="img"
        :aria-label="summary"
        class="aspect-square size-28 shrink-0"
      >
        <VisSingleContainer :data="arcs" :height="112">
          <VisDonut
            :value="(d: Arc['data']) => d.value"
            :color="(d: Arc['data']) => `var(--color-${d.id})`"
            :arc-width="22"
            :pad-angle="0.03"
            :corner-radius="2"
            :sort-function="() => 0"
          />
          <ChartTooltip :triggers="triggers" />
        </VisSingleContainer>
      </ChartContainer>
      <ul class="flex min-w-40 flex-1 flex-col gap-1 text-xs">
        <li v-for="entry in entries" :key="entry.id" class="flex items-center gap-2">
          <svg class="size-2.5 shrink-0" viewBox="0 0 10 10" aria-hidden="true">
            <circle cx="5" cy="5" r="5" :style="{ fill: entry.color }" />
          </svg>
          <span class="min-w-0 flex-1 break-words">{{ entry.label }}</span>
          <span class="font-medium tabular-nums">{{ n(entry.value, 'int') }}</span>
          <span class="text-muted-foreground w-12 text-right tabular-nums">{{
            n(entry.percent, 'percent')
          }}</span>
        </li>
      </ul>
    </div>
  </figure>
</template>
