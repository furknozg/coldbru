<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'

import { getLlmSeries, getOverview, getPromptVersionUsage } from '@/api/analytics/service'
import { LazyDistributionPie } from '@/components/LazyDistributionPie'
import { modelColors, purposeColor } from '@/features/dashboard/chart'
import DashboardPage from '@/features/dashboard/components/DashboardPage.vue'
import DateRangeFilter from '@/features/dashboard/components/DateRangeFilter.vue'
import { LazyBarChart, LazyTimeSeriesChart } from '@/features/dashboard/components/LazyCharts'
import PromptVersionTable from '@/features/dashboard/components/PromptVersionTable.vue'
import ResourceState from '@/features/dashboard/components/ResourceState.vue'
import SegmentedToggle from '@/features/dashboard/components/SegmentedToggle.vue'
import StatList from '@/features/dashboard/components/StatList.vue'
import { cacheTokens, modelSlices, purposeSlices } from '@/features/dashboard/overview'
import {
  OTHER_KEY,
  llmCallsLine,
  llmCostLines,
  modelKeys,
  toNumber,
} from '@/features/dashboard/series'
import { useDateRange } from '@/features/dashboard/useDateRange'
import { useResource } from '@/features/dashboard/useResource'

/**
 * `/analytics/llm` (D-097): what the model calls cost and how well the prompts do over the chosen
 * range. Figures and pies come from `/overview`, the table and its chart from `/llm-usage`, the
 * charts over time from `/llm-usage/series`; all follow the date filter in the URL.
 */
const { t, n } = useI18n()
const { range, preset, apiRange, series, select, selectPreset } = useDateRange()

const overview = useResource(apiRange, getOverview)
const versions = useResource(apiRange, getPromptVersionUsage)
const over = useResource(series, getLlmSeries)

const llm = computed(() => overview.data.value?.llm ?? null)

const stats = computed(() => {
  const l = llm.value
  if (!l) return []
  return [
    { key: 'calls', label: t('dashboard.stat.llmCalls'), value: n(l.calls, 'int') },
    { key: 'cost', label: t('dashboard.stat.llmCost'), value: n(toNumber(l.cost_usd), 'usd') },
    { key: 'input', label: t('dashboard.stat.inputTokens'), value: n(l.tokens.input, 'int') },
    { key: 'output', label: t('dashboard.stat.outputTokens'), value: n(l.tokens.output, 'int') },
    {
      key: 'cache',
      label: t('dashboard.stat.cache'),
      value: n(cacheTokens(l), 'int'),
      note: t('dashboard.stat.cacheSplit', {
        read: n(l.tokens.cache_read, 'int'),
        creation: n(l.tokens.cache_creation, 'int'),
      }),
    },
  ]
})

const purposeLabel = (purpose: string) => t(`dashboard.purpose.${purpose}`)

// Versions bar chart: one measure at a time, so the y axis never mixes percent and milliseconds.
type Measure = 'repairRate' | 'latency'
const measure = shallowRef<Measure>('repairRate')
const measures = computed(() => [
  { value: 'repairRate' as const, label: t('dashboard.llm.repairRate') },
  { value: 'latency' as const, label: t('dashboard.llm.latency') },
])
const bars = computed(() =>
  (versions.data.value ?? []).map((row) => ({
    key: `${row.prompt_name}@${row.prompt_version}`,
    label: `${row.prompt_name} ${row.prompt_version}`,
    value: measure.value === 'repairRate' ? row.repair_rate : row.avg_latency_ms,
  })),
)
const formatBar = (value: number) =>
  measure.value === 'repairRate'
    ? n(value, 'percent')
    : t('dashboard.llm.ms', { value: n(value, 'int') })
const measureLabel = computed(() => measures.value.find((m) => m.value === measure.value)!.label)

// Cost over time, stacked by purpose or by model.
type Split = 'purpose' | 'model'
const split = shallowRef<Split>('purpose')
const splits = computed(() => [
  { value: 'purpose' as const, label: t('dashboard.llm.byPurpose') },
  { value: 'model' as const, label: t('dashboard.llm.byModel') },
])
const costLines = computed(() => {
  const s = over.data.value
  if (!s) return []
  const lines = llmCostLines(s, split.value)
  if (split.value === 'purpose') {
    return lines.map((line) => ({
      ...line,
      label: purposeLabel(line.key),
      color: purposeColor(line.key),
    }))
  }
  const colors = modelColors(modelKeys(s))
  return lines.map((line) => ({
    ...line,
    label: line.key === OTHER_KEY ? t('chart.other') : line.key,
    color: colors[line.key]!,
  }))
})
const callLines = computed(() => {
  const s = over.data.value
  return s
    ? [{ ...llmCallsLine(s), label: t('dashboard.stat.llmCalls'), color: 'var(--chart-1)' }]
    : []
})
const windowNote = computed(() => (range.value.from ? '' : ` · ${t('dashboard.range.last90')}`))
</script>

<template>
  <DashboardPage :title="t('dashboard.menu.llm')">
    <template #actions>
      <DateRangeFilter :range="range" :preset="preset" @preset="selectPreset" @range="select" />
    </template>

    <ResourceState
      :loading="overview.loading.value"
      :error="overview.error.value"
      :ready="Boolean(llm)"
      @retry="overview.reload()"
    >
      <template v-if="llm">
        <StatList :items="stats" />
        <div class="grid gap-6 md:grid-cols-2">
          <LazyDistributionPie
            :title="t('dashboard.llm.costByPurpose')"
            format="usd"
            :slices="purposeSlices(llm.by_purpose, purposeLabel)"
          />
          <LazyDistributionPie
            :title="t('dashboard.llm.costByModel')"
            format="usd"
            :slices="modelSlices(llm.by_model, t('chart.other'))"
          />
        </div>
      </template>
    </ResourceState>

    <section class="flex flex-col gap-3" :aria-label="t('dashboard.llm.versions')">
      <h3 class="text-espresso text-base font-semibold">{{ t('dashboard.llm.versions') }}</h3>
      <ResourceState
        :loading="versions.loading.value"
        :error="versions.error.value"
        :ready="Boolean(versions.data.value)"
        :empty="versions.data.value?.length === 0 ? t('dashboard.llm.noVersions') : undefined"
        @retry="versions.reload()"
      >
        <template v-if="versions.data.value">
          <PromptVersionTable :rows="versions.data.value" />
          <SegmentedToggle
            v-model="measure"
            :label="t('dashboard.llm.measure')"
            :options="measures"
          />
          <LazyBarChart
            :title="t('dashboard.llm.versionChart')"
            :measure="measureLabel"
            :bars="bars"
            :format-value="formatBar"
          />
        </template>
      </ResourceState>
    </section>

    <section class="flex flex-col gap-3" :aria-label="t('dashboard.llm.overTime')">
      <h3 class="text-espresso text-base font-semibold">
        {{ t('dashboard.llm.overTime') }}{{ windowNote }}
      </h3>
      <ResourceState
        :loading="over.loading.value"
        :error="over.error.value"
        :ready="Boolean(over.data.value)"
        @retry="over.reload()"
      >
        <template v-if="over.data.value">
          <SegmentedToggle v-model="split" :label="t('dashboard.llm.split')" :options="splits" />
          <LazyTimeSeriesChart
            :title="t('dashboard.llm.costOverTime')"
            :buckets="over.data.value.buckets"
            :bucket="over.data.value.bucket"
            :series="costLines"
            kind="area"
            format="usd"
          />
          <LazyTimeSeriesChart
            :title="t('dashboard.llm.callsOverTime')"
            :buckets="over.data.value.buckets"
            :bucket="over.data.value.bucket"
            :series="callLines"
            kind="line"
            format="int"
          />
        </template>
      </ResourceState>
    </section>
  </DashboardPage>
</template>
