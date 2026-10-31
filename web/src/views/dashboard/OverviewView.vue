<script setup lang="ts">
import { ArrowRightIcon } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { getOverview, getOverviewSeries } from '@/api/analytics/service'
import { LazyDistributionPie } from '@/components/LazyDistributionPie'
import DashboardPage from '@/features/dashboard/components/DashboardPage.vue'
import DateRangeFilter from '@/features/dashboard/components/DateRangeFilter.vue'
import { LazyTimeSeriesChart } from '@/features/dashboard/components/LazyCharts'
import ResourceState from '@/features/dashboard/components/ResourceState.vue'
import StatList from '@/features/dashboard/components/StatList.vue'
import { channelColor } from '@/features/dashboard/chart'
import {
  channelSlices,
  deliveriesWithStatus,
  cacheTokens,
  statusSlices,
  totalTokens,
} from '@/features/dashboard/overview'
import { clickLines, deliveryLines, toNumber } from '@/features/dashboard/series'
import { useDateRange } from '@/features/dashboard/useDateRange'
import { useResource } from '@/features/dashboard/useResource'

/**
 * `/` (D-097): the entry of the app. One card leads to the chat for a new campaign; below it the
 * figures of the chosen date range, how the deliveries and the LLM spend split, and the deliveries
 * over time. The three requests follow the date filter in the URL.
 */
const { t, n } = useI18n()
const { range, preset, apiRange, series, select, selectPreset } = useDateRange()

const overview = useResource(apiRange, getOverview)
const deliveries = useResource(series, getOverviewSeries)

const stats = computed(() => {
  const o = overview.data.value
  if (!o) return []
  return [
    {
      key: 'campaigns',
      label: t('dashboard.stat.campaigns'),
      value: n(o.campaigns_completed, 'int'),
    },
    {
      key: 'sent',
      label: t('dashboard.stat.sent'),
      value: n(deliveriesWithStatus(o.deliveries, 'sent'), 'int'),
    },
    {
      key: 'failed',
      label: t('dashboard.stat.failed'),
      value: n(deliveriesWithStatus(o.deliveries, 'failed'), 'int'),
    },
    {
      key: 'deliveryCost',
      label: t('dashboard.stat.deliveryCost'),
      value: n(toNumber(o.delivery_cost_usd), 'usd'),
    },
    {
      key: 'ctr',
      label: t('dashboard.stat.ctr'),
      // Nothing sent in the range: a rate would be a made-up 0%.
      value: o.clicks.sent > 0 ? n(o.clicks.ctr, 'percent') : t('dashboard.click.unknown'),
      note: t('dashboard.stat.ctrSplit', {
        unique: n(o.clicks.unique_clicks, 'int'),
        total: n(o.clicks.total_clicks, 'int'),
      }),
    },
    { key: 'llmCalls', label: t('dashboard.stat.llmCalls'), value: n(o.llm.calls, 'int') },
    {
      key: 'llmCost',
      label: t('dashboard.stat.llmCost'),
      value: n(toNumber(o.llm.cost_usd), 'usd'),
    },
    {
      key: 'tokens',
      label: t('dashboard.stat.tokens'),
      value: n(totalTokens(o.llm), 'int'),
      note: t('dashboard.stat.tokenSplit', {
        input: n(o.llm.tokens.input, 'int'),
        output: n(o.llm.tokens.output, 'int'),
      }),
    },
    {
      key: 'cache',
      label: t('dashboard.stat.cache'),
      value: n(cacheTokens(o.llm), 'int'),
      note: t('dashboard.stat.cacheSplit', {
        read: n(o.llm.tokens.cache_read, 'int'),
        creation: n(o.llm.tokens.cache_creation, 'int'),
      }),
    },
  ]
})

const channelLabel = (channel: string) => t(`channel.${channel}`)
const statusLabel = (status: string) => t(`dashboard.deliveryStatus.${status}`)

const lines = computed(() => {
  const s = deliveries.data.value
  if (!s) return []
  return deliveryLines(s).map((line) => ({
    ...line,
    label: channelLabel(line.key),
    color: channelColor(line.key),
  }))
})

const clickChartLines = computed(() => {
  const s = deliveries.data.value
  if (!s) return []
  return clickLines(s).map((line) => ({
    ...line,
    label: channelLabel(line.key),
    color: channelColor(line.key),
  }))
})

// "Tümü" and a range without a start draw the last 90 days, which the title says.
const seriesTitle = computed(() =>
  range.value.from ? t('dashboard.overview.series') : t('dashboard.overview.seriesRecent'),
)
const clickSeriesTitle = computed(() =>
  range.value.from
    ? t('dashboard.overview.clickSeries')
    : t('dashboard.overview.clickSeriesRecent'),
)
</script>

<template>
  <DashboardPage :title="t('dashboard.menu.overview')">
    <template #actions>
      <DateRangeFilter :range="range" :preset="preset" @preset="selectPreset" @range="select" />
    </template>

    <RouterLink
      :to="{ name: 'new-campaign' }"
      data-slot="card"
      class="bg-card hover:bg-accent focus-visible:ring-ring/50 no-underline! flex items-center justify-between gap-4 rounded-lg border p-4 outline-none focus-visible:ring-3"
    >
      <span class="flex flex-col gap-0.5">
        <span class="text-espresso font-semibold">{{ t('dashboard.create.title') }}</span>
        <span class="text-muted-foreground text-sm">{{ t('dashboard.create.description') }}</span>
      </span>
      <ArrowRightIcon class="text-caramel size-5 shrink-0" aria-hidden="true" />
    </RouterLink>

    <ResourceState
      :loading="overview.loading.value"
      :error="overview.error.value"
      :ready="Boolean(overview.data.value)"
      @retry="overview.reload()"
    >
      <template v-if="overview.data.value">
        <StatList :items="stats" />
        <div class="grid gap-6 md:grid-cols-2">
          <LazyDistributionPie
            :title="t('dashboard.overview.byChannel')"
            :slices="channelSlices(overview.data.value.deliveries, channelLabel)"
          />
          <LazyDistributionPie
            :title="t('dashboard.overview.byStatus')"
            :slices="statusSlices(overview.data.value.deliveries, statusLabel)"
          />
        </div>
      </template>
    </ResourceState>

    <ResourceState
      :loading="deliveries.loading.value"
      :error="deliveries.error.value"
      :ready="Boolean(deliveries.data.value)"
      @retry="deliveries.reload()"
    >
      <LazyTimeSeriesChart
        v-if="deliveries.data.value"
        :title="seriesTitle"
        :buckets="deliveries.data.value.buckets"
        :bucket="deliveries.data.value.bucket"
        :series="lines"
        kind="line"
        format="int"
      />
      <LazyTimeSeriesChart
        v-if="deliveries.data.value"
        :title="clickSeriesTitle"
        :buckets="deliveries.data.value.buckets"
        :bucket="deliveries.data.value.bucket"
        :series="clickChartLines"
        kind="line"
        format="int"
      />
    </ResourceState>
  </DashboardPage>
</template>
