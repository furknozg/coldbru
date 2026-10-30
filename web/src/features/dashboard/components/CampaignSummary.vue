<script setup lang="ts">
import { computed, type Component } from 'vue'
import { useI18n } from 'vue-i18n'

import type { CampaignReport } from '@/api/analytics/service'
import { LazyDistributionPie } from '@/components/LazyDistributionPie'

import { channelSlices, purposeSlices, statusSlices, totalTokens } from '../overview'
import { PURPOSES, toNumber } from '../series'
import StatList from './StatList.vue'

/**
 * What Analytics knows of one campaign (D-083, D-097): when it completed, the cost of its
 * deliveries and of its model calls, how the deliveries ended and which step the model spent on.
 * Later metric blocks (T-044) are added as `sections`: each is rendered under its title with the
 * report as its only prop, so this component does not change when a block is added.
 */
export interface SummarySection {
  key: string
  title: string
  component: Component
}

const props = withDefaults(
  defineProps<{ report: CampaignReport; sections?: readonly SummarySection[] }>(),
  {
    sections: () => [],
  },
)

const { t, n, d } = useI18n()

const stats = computed(() => {
  const { report } = props
  return [
    {
      key: 'deliveryCost',
      label: t('dashboard.stat.deliveryCost'),
      value: n(toNumber(report.delivery_cost_usd), 'usd'),
    },
    { key: 'segments', label: t('dashboard.stat.segments'), value: n(report.sms_segments, 'int') },
    { key: 'llmCalls', label: t('dashboard.stat.llmCalls'), value: n(report.llm.calls, 'int') },
    {
      key: 'llmCost',
      label: t('dashboard.stat.llmCost'),
      value: n(toNumber(report.llm.cost_usd), 'usd'),
    },
    {
      key: 'tokens',
      label: t('dashboard.stat.tokens'),
      value: n(totalTokens(report.llm), 'int'),
      note: t('dashboard.stat.tokenSplit', {
        input: n(report.llm.tokens.input, 'int'),
        output: n(report.llm.tokens.output, 'int'),
      }),
    },
  ]
})

// A completed event carries `completed_at`; before it, the campaign has LLM usage but no deliveries.
const completedAt = computed(() => props.report.completed_at || null)
const channelLabel = (channel: string) => t(`channel.${channel}`)
const statusLabel = (status: string) => t(`dashboard.deliveryStatus.${status}`)
const purposeLabel = (purpose: string) => t(`dashboard.purpose.${purpose}`)
// The table follows the order of the flow, like the pie.
const purposeRows = computed(() =>
  PURPOSES.flatMap((purpose) =>
    props.report.llm.by_purpose.filter((row) => row.purpose === purpose),
  ),
)
</script>

<template>
  <div class="flex flex-col gap-6">
    <p class="text-muted-foreground text-sm" data-testid="completed-at">
      <template v-if="completedAt">
        {{ t('dashboard.campaign.completedAt') }}
        <time class="text-foreground" :datetime="completedAt">{{ d(completedAt, 'short') }}</time>
      </template>
      <template v-else>{{ t('dashboard.campaign.notCompleted') }}</template>
    </p>

    <StatList :items="stats" />

    <div v-if="completedAt" class="grid gap-6 md:grid-cols-2">
      <LazyDistributionPie
        :title="t('dashboard.overview.byChannel')"
        :slices="channelSlices(report.deliveries, channelLabel)"
      />
      <LazyDistributionPie
        :title="t('dashboard.overview.byStatus')"
        :slices="statusSlices(report.deliveries, statusLabel)"
      />
    </div>

    <section
      v-if="report.llm.by_purpose.length"
      class="flex flex-col gap-3"
      :aria-label="t('dashboard.campaign.llmByPurpose')"
    >
      <h3 class="text-espresso text-base font-semibold">
        {{ t('dashboard.campaign.llmByPurpose') }}
      </h3>
      <div class="grid items-start gap-6 md:grid-cols-2">
        <LazyDistributionPie
          :title="t('dashboard.llm.costByPurpose')"
          format="usd"
          :slices="purposeSlices(report.llm.by_purpose, purposeLabel)"
        />
        <table class="w-full text-sm">
          <caption class="sr-only">
            {{
              t('dashboard.campaign.llmByPurpose')
            }}
          </caption>
          <thead>
            <tr class="text-muted-foreground border-b text-left text-xs">
              <th scope="col" class="py-1.5 pr-3 font-medium">{{ t('dashboard.llm.purpose') }}</th>
              <th scope="col" class="px-3 py-1.5 text-right font-medium">
                {{ t('dashboard.llm.calls') }}
              </th>
              <th scope="col" class="px-3 py-1.5 text-right font-medium">
                {{ t('dashboard.llm.tokens') }}
              </th>
              <th scope="col" class="py-1.5 pl-3 text-right font-medium">
                {{ t('dashboard.llm.cost') }}
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="row in purposeRows"
              :key="row.purpose"
              class="border-b last:border-b-0"
              data-row="purpose"
            >
              <th scope="row" class="py-1.5 pr-3 text-left font-medium">
                {{ purposeLabel(row.purpose) }}
              </th>
              <td class="px-3 py-1.5 text-right tabular-nums">{{ n(row.calls, 'int') }}</td>
              <td class="px-3 py-1.5 text-right tabular-nums">
                {{ n(row.input_tokens + row.output_tokens, 'int') }}
              </td>
              <td class="py-1.5 pl-3 text-right tabular-nums">
                {{ n(toNumber(row.cost_usd), 'usd') }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section
      v-for="section in sections"
      :key="section.key"
      class="flex flex-col gap-3"
      :data-section="section.key"
      :aria-label="section.title"
    >
      <h3 class="text-espresso text-base font-semibold">{{ section.title }}</h3>
      <component :is="section.component" :report="report" />
    </section>
  </div>
</template>
