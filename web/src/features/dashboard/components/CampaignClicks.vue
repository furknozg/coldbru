<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { CampaignReport } from '@/api/analytics/service'

import StatList from './StatList.vue'

/**
 * "Başarı metrikleri" of a campaign (D-108), rendered through the `sections` of `CampaignSummary`.
 * CTR is unique clickers over sent deliveries and the sent counts arrive with the completed event,
 * so before completion the clicks are shown live and CTR (and `sent`) are left as a dash, never a
 * made-up 0%.
 */
const props = defineProps<{ report: CampaignReport }>()

const { t, n } = useI18n()

const clicks = computed(() => props.report.clicks)
const completed = computed(() => Boolean(props.report.completed_at))
const empty = computed(
  () => !completed.value && clicks.value.total_clicks === 0 && clicks.value.by_locale.length === 0,
)

const percent = (ctr: number) => n(ctr, 'percent')
const dash = computed(() => t('dashboard.click.unknown'))

const stats = computed(() => [
  ...(completed.value
    ? [
        {
          key: 'ctr',
          label: t('dashboard.click.ctr'),
          value: percent(clicks.value.ctr),
          note: t('dashboard.click.ctrNote', {
            unique: n(clicks.value.unique_clicks, 'int'),
            sent: n(clicks.value.sent, 'int'),
          }),
        },
      ]
    : []),
  {
    key: 'uniqueClicks',
    label: t('dashboard.click.uniqueClicks'),
    value: n(clicks.value.unique_clicks, 'int'),
  },
  {
    key: 'totalClicks',
    label: t('dashboard.click.totalClicks'),
    value: n(clicks.value.total_clicks, 'int'),
  },
])

const channelStats = computed(() =>
  completed.value
    ? clicks.value.by_channel.map((row) => ({
        key: `ctr-${row.channel}`,
        label: `${t('dashboard.click.byChannel')} · ${t(`channel.${row.channel}`)}`,
        value: percent(row.ctr),
        note: t('dashboard.click.ctrNote', {
          unique: n(row.unique_clicks, 'int'),
          sent: n(row.sent, 'int'),
        }),
      }))
    : [],
)
</script>

<template>
  <div class="flex flex-col gap-4" data-testid="click-metrics">
    <p v-if="!completed" class="text-muted-foreground text-sm" data-testid="ctr-pending">
      {{ t('dashboard.click.pending') }}
    </p>
    <p v-if="empty" class="text-muted-foreground text-sm" data-testid="clicks-empty">
      {{ t('dashboard.click.none') }}
    </p>
    <template v-else>
      <StatList :items="stats" />
      <StatList v-if="channelStats.length" :items="channelStats" />

      <div v-if="clicks.by_locale.length" class="flex flex-col gap-2">
        <h4 class="text-muted-foreground text-xs font-medium">
          {{ t('dashboard.click.byLocale') }}
        </h4>
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <caption class="sr-only">
              {{
                t('dashboard.click.byLocale')
              }}
            </caption>
            <thead>
              <tr class="text-muted-foreground border-b text-left text-xs">
                <th scope="col" class="py-1.5 pr-3 font-medium">
                  {{ t('dashboard.click.channel') }}
                </th>
                <th scope="col" class="px-3 py-1.5 font-medium">
                  {{ t('dashboard.click.language') }}
                </th>
                <th scope="col" class="px-3 py-1.5 font-medium">
                  {{ t('dashboard.click.timezone') }}
                </th>
                <th scope="col" class="px-3 py-1.5 text-right font-medium">
                  {{ t('dashboard.click.sent') }}
                </th>
                <th scope="col" class="px-3 py-1.5 text-right font-medium">
                  {{ t('dashboard.click.uniqueClicks') }}
                </th>
                <th scope="col" class="px-3 py-1.5 text-right font-medium">
                  {{ t('dashboard.click.totalClicks') }}
                </th>
                <th scope="col" class="py-1.5 pl-3 text-right font-medium">
                  {{ t('dashboard.click.ctr') }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in clicks.by_locale"
                :key="`${row.channel}|${row.language}|${row.timezone}`"
                class="border-b last:border-b-0"
                data-row="locale"
              >
                <th scope="row" class="py-1.5 pr-3 text-left font-medium">
                  {{ t(`channel.${row.channel}`) }}
                </th>
                <td class="px-3 py-1.5">{{ t(`language.${row.language}`) }}</td>
                <td class="px-3 py-1.5">{{ row.timezone }}</td>
                <td class="px-3 py-1.5 text-right tabular-nums">
                  {{ completed ? n(row.sent, 'int') : dash }}
                </td>
                <td class="px-3 py-1.5 text-right tabular-nums">
                  {{ n(row.unique_clicks, 'int') }}
                </td>
                <td class="px-3 py-1.5 text-right tabular-nums">
                  {{ n(row.total_clicks, 'int') }}
                </td>
                <td class="py-1.5 pl-3 text-right tabular-nums">
                  {{ completed ? percent(row.ctr) : dash }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
  </div>
</template>
