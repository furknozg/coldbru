<script setup lang="ts">
import { CurveType } from '@unovis/ts'
import { VisArea, VisAxis, VisCrosshair, VisLine, VisTooltip, VisXYContainer } from '@unovis/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { ChartContainer, type ChartConfig } from '@/components/ui/chart'

import { tooltipHtml } from '../chart'
import type { SeriesLine } from '../series'

/**
 * Values over time (D-097): lines for counts, stacked areas for a cost split by purpose or model.
 * `buckets` are the UTC starts of every bucket and each series has one value per bucket, so the
 * x axis is the bucket index and the labels are formatted in the viewer's time zone. The legend
 * is real text; hovering shows every series of the bucket. A chart without any value is a line of
 * text instead, never an empty plot. The chart draws no caption of its own: `title` names it in
 * the summary for assistive technology, and the card around it shows the same text as its
 * heading (D-111), so the title is never on the screen twice.
 */
const props = defineProps<{
  /** What the chart shows; the heading of the card it sits in. */
  title: string
  buckets: readonly string[]
  bucket: 'hour' | 'day'
  series: readonly (SeriesLine & { label: string; color: string })[]
  kind: 'line' | 'area'
  /** The `numberFormats` entry for values; ticks use its compact twin for money. */
  format: 'int' | 'usd'
}>()

const { t, n, d } = useI18n()

interface Row {
  index: number
  values: number[]
}

const rows = computed<Row[]>(() =>
  props.buckets.map((_, index) => ({
    index,
    values: props.series.map((s) => s.values[index] ?? 0),
  })),
)
const hasValues = computed(() => props.series.some((s) => s.values.some((v) => v > 0)))

const colors = computed(() => props.series.map((s) => s.color))
const yAccessors = computed(() => props.series.map((_, k) => (row: Row) => row.values[k] ?? 0))

const config = computed<ChartConfig>(() =>
  Object.fromEntries(props.series.map((s) => [s.key, { label: s.label, color: s.color }])),
)

const whenAt = (index: number) => new Date(props.buckets[index] ?? 0)
const tickLabel = (index: number) =>
  props.buckets[index] ? d(whenAt(index), props.bucket === 'hour' ? 'hourLabel' : 'dayLabel') : ''
const fullLabel = (index: number) => d(whenAt(index), props.bucket === 'hour' ? 'short' : 'date')

// About seven labels fit under the plot; the rest of the buckets only get a tick.
const tickValues = computed(() => {
  const step = Math.max(1, Math.ceil(props.buckets.length / 7))
  return props.buckets.map((_, index) => index).filter((index) => index % step === 0)
})

const valueText = (value: number) => n(value, props.format)
const axisText = (value: number) => n(value, props.format === 'usd' ? 'usdTick' : 'compact')

const summary = computed(() =>
  t('chart.summary', {
    title: props.title,
    items: props.series
      .map((s) => `${s.label}: ${valueText(s.values.reduce((sum, v) => sum + v, 0))}`)
      .join(', '),
  }),
)

const crosshairTemplate = (row: Row) =>
  tooltipHtml(
    fullLabel(row.index),
    props.series.map((s, k) => ({
      label: s.label,
      color: s.color,
      value: valueText(row.values[k] ?? 0),
    })),
  )
const crosshairColor = (_row: Row, k: number) => colors.value[k] ?? 'currentColor'

// Surface ring of the crosshair dot and a recessive grid, set where unovis reads its CSS variables.
const plotStyle = {
  '--vis-crosshair-circle-stroke-color': 'var(--background)',
  '--vis-crosshair-line-stroke-color': 'var(--border)',
  '--vis-axis-grid-color': 'var(--border)',
  '--vis-axis-domain-color': 'var(--border)',
}
</script>

<template>
  <figure class="flex min-w-0 flex-col gap-2" :aria-label="title">
    <p v-if="!hasValues" class="text-muted-foreground text-xs">{{ t('chart.noData') }}</p>
    <template v-else>
      <ChartContainer
        :config="config"
        cursor
        role="img"
        :aria-label="summary"
        class="aspect-auto h-52 w-full"
      >
        <div :style="plotStyle" class="h-full w-full">
          <VisXYContainer
            :data="rows"
            :y-domain="[0, undefined]"
            :margin="{ top: 8, right: 8 }"
            :height="208"
          >
            <VisArea
              v-if="kind === 'area'"
              :x="(row: Row) => row.index"
              :y="yAccessors"
              :color="colors"
              :opacity="0.85"
              :curve-type="CurveType.Linear"
            />
            <VisLine
              v-else
              :x="(row: Row) => row.index"
              :y="yAccessors"
              :color="colors"
              :line-width="2"
              :curve-type="CurveType.Linear"
            />
            <VisAxis
              type="x"
              :tick-values="tickValues"
              :tick-format="(tick: number | Date) => tickLabel(Number(tick))"
              :grid-line="false"
              :tick-line="false"
              :tick-text-hide-overlapping="true"
            />
            <VisAxis
              type="y"
              :num-ticks="4"
              :tick-format="(tick: number | Date) => axisText(Number(tick))"
              :tick-line="false"
              :domain-line="false"
            />
            <VisCrosshair
              :x="(row: Row) => row.index"
              :y="yAccessors"
              :color="crosshairColor"
              :template="crosshairTemplate"
            />
            <VisTooltip />
          </VisXYContainer>
        </div>
      </ChartContainer>
      <ul v-if="series.length > 1" class="flex flex-wrap gap-x-4 gap-y-1 text-xs">
        <li v-for="s in series" :key="s.key" class="flex items-center gap-1.5">
          <svg class="size-2.5 shrink-0" viewBox="0 0 10 10" aria-hidden="true">
            <circle cx="5" cy="5" r="5" :style="{ fill: s.color }" />
          </svg>
          {{ s.label }}
        </li>
      </ul>
    </template>
  </figure>
</template>
