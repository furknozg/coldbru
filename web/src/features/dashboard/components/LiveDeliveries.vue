<script setup lang="ts">
import { computed, toRef } from 'vue'
import { useI18n } from 'vue-i18n'

import { LIVE_CHANNELS, LIVE_STATUSES, countOf, progressOf } from '../live'
import { useLiveDeliveries } from '../useLiveDeliveries'
import MetricCard from './MetricCard.vue'
import MetricSkeleton from './MetricSkeleton.vue'

/**
 * The live counts of one campaign (D-080, D-097): a progress bar of the deliveries that reached an
 * end state, the counts per channel and status, and the state of the connection. The stream opens
 * with the component (or when the campaign changes) and closes when it goes away. The progress
 * and the table are one card each, under their own titles; before the first snapshot they are a
 * skeleton (D-111).
 */
const props = defineProps<{ campaignId: string }>()

const { t, n } = useI18n()
const { summary, done, status } = useLiveDeliveries(toRef(props, 'campaignId'))

const progress = computed(() => (summary.value ? progressOf(summary.value) : null))
const percent = computed(() => Math.round((progress.value?.ratio ?? 0) * 100))

// What the line above the bar says: the end of the campaign, or how the connection is doing.
const connection = computed(() => {
  if (done.value) return t(`dashboard.live.done.${done.value}`)
  if (status.value === 'open') return t('dashboard.live.open')
  if (status.value === 'error') return t('dashboard.live.error')
  return t('dashboard.live.connecting')
})
</script>

<template>
  <section class="flex flex-col gap-4" :aria-label="t('dashboard.menu.live')">
    <p
      class="text-muted-foreground flex items-center gap-2 text-sm"
      data-testid="connection"
      :data-state="done ?? status"
      role="status"
    >
      <span
        class="size-2 shrink-0 rounded-full"
        :class="status === 'open' && !done ? 'bg-chart-2' : 'bg-muted-foreground/40'"
        aria-hidden="true"
      />
      {{ connection }}
    </p>

    <template v-if="summary && progress">
      <div
        class="flex flex-col gap-4 motion-safe:animate-in motion-safe:fade-in motion-safe:duration-300"
      >
        <MetricCard :title="t('dashboard.live.progressLabel')">
          <div class="flex flex-col gap-1.5">
            <div class="flex items-baseline justify-between gap-3 text-sm">
              <span>
                {{
                  t('dashboard.live.progress', {
                    finished: n(progress.finished, 'int'),
                    total: n(summary.total, 'int'),
                  })
                }}
              </span>
              <span class="text-muted-foreground tabular-nums">{{
                n(progress.ratio, 'percent')
              }}</span>
            </div>
            <div
              class="bg-border h-2 w-full overflow-hidden rounded-full"
              role="progressbar"
              :aria-label="t('dashboard.live.progressLabel')"
              aria-valuemin="0"
              aria-valuemax="100"
              :aria-valuenow="percent"
            >
              <div
                class="bg-primary h-full rounded-full motion-safe:transition-[width]"
                :style="{ width: `${percent}%` }"
              />
            </div>
          </div>
        </MetricCard>

        <MetricCard :title="t('dashboard.live.deliveries')">
          <div class="overflow-x-auto">
            <table class="w-full text-sm">
              <caption class="sr-only">
                {{
                  t('dashboard.live.counts')
                }}
              </caption>
              <thead>
                <tr class="text-muted-foreground border-b text-left text-xs">
                  <th scope="col" class="py-1.5 pr-3 font-medium">
                    {{ t('dashboard.live.channel') }}
                  </th>
                  <th
                    v-for="status in LIVE_STATUSES"
                    :key="status"
                    scope="col"
                    class="px-3 py-1.5 text-right font-medium"
                  >
                    {{ t(`dashboard.deliveryStatus.${status}`) }}
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="channel in LIVE_CHANNELS"
                  :key="channel"
                  :data-channel="channel"
                  class="border-b last:border-b-0"
                >
                  <th scope="row" class="py-1.5 pr-3 text-left font-medium">
                    {{ t(`channel.${channel}`) }}
                  </th>
                  <td
                    v-for="status in LIVE_STATUSES"
                    :key="status"
                    :data-status="status"
                    class="px-3 py-1.5 text-right tabular-nums"
                  >
                    {{ n(countOf(summary, channel, status), 'int') }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </MetricCard>
      </div>
    </template>
    <template v-else>
      <p class="text-muted-foreground text-sm">{{ t('dashboard.live.waiting') }}</p>
      <MetricSkeleton :tiles="0" :rows="3" />
    </template>
  </section>
</template>
