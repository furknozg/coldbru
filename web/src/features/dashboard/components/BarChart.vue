<script setup lang="ts">
import { VisAxis, VisGroupedBar, VisTooltip, VisXYContainer } from '@unovis/vue'
import { GroupedBar } from '@unovis/ts'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { ChartContainer, type ChartConfig } from '@/components/ui/chart'

import { tooltipHtml } from '../chart'

/**
 * One value per category as columns (D-097): the prompt versions with their repair rate or mean
 * latency. Columns are at most 24px wide with a 4px rounded top. The exact value is in the
 * tooltip and in the label above the column; the table next to the chart holds every figure.
 * Like the time series chart, it draws no caption: `title` names it in the summary and the card
 * around it shows the same text as its heading (D-111).
 */
const props = defineProps<{
  /** What the columns show; the heading of the card it sits in. */
  title: string
  /** Name of what the columns measure, for the tooltip and the summary. */
  measure: string
  bars: readonly { key: string; label: string; value: number }[]
  /** Text of one value, e.g. "%6,3" or "812 ms". */
  formatValue: (value: number) => string
  color?: string
}>()

const { t } = useI18n()

interface Bar {
  index: number
  label: string
  value: number
}

const rows = computed<Bar[]>(() =>
  props.bars.map((bar, index) => ({ index, label: bar.label, value: bar.value })),
)
const hasValues = computed(() => props.bars.some((bar) => bar.value > 0))
const color = computed(() => props.color ?? 'var(--chart-1)')

const config = computed<ChartConfig>(() => ({
  value: { label: props.measure, color: color.value },
}))

const summary = computed(() =>
  t('chart.summary', {
    title: `${props.title} (${props.measure})`,
    items: props.bars.map((b) => `${b.label}: ${props.formatValue(b.value)}`).join(', '),
  }),
)

const triggers = {
  [GroupedBar.selectors.bar]: (row: Bar) =>
    tooltipHtml(row.label, [
      { label: props.measure, color: color.value, value: props.formatValue(row.value) },
    ]),
}

const plotStyle = {
  '--vis-axis-grid-color': 'var(--border)',
  '--vis-axis-domain-color': 'var(--border)',
}
</script>

<template>
  <figure class="flex min-w-0 flex-col gap-2" :aria-label="title">
    <p v-if="!hasValues" class="text-muted-foreground text-xs">{{ t('chart.noData') }}</p>
    <ChartContainer
      v-else
      :config="config"
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
          <VisGroupedBar
            :x="(row: Bar) => row.index"
            :y="(row: Bar) => row.value"
            :color="color"
            :rounded-corners="4"
            :group-max-width="24"
            :bar-padding="0.4"
            :bar-min-height="0"
          />
          <VisAxis
            type="x"
            :tick-values="rows.map((row) => row.index)"
            :tick-format="(tick: number | Date) => bars[Number(tick)]?.label ?? ''"
            :grid-line="false"
            :tick-line="false"
            :tick-text-width="160"
            :tick-text-angle="-30"
            :tick-text-align="'right'"
          />
          <VisAxis
            type="y"
            :num-ticks="4"
            :tick-format="(tick: number | Date) => formatValue(Number(tick))"
            :tick-line="false"
            :domain-line="false"
          />
          <VisTooltip :triggers="triggers" />
        </VisXYContainer>
      </div>
    </ChartContainer>
  </figure>
</template>
