<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { CampaignReport } from '@/api/analytics/service'

/** The LLM figures of the selected campaign; Analytics has none until the campaign is processed. */
defineProps<{
  report: CampaignReport | null
  loading: boolean
  failed: boolean
}>()

const { t, n } = useI18n()
</script>

<template>
  <section class="bg-muted/60 rounded-lg border p-3 text-sm" :aria-label="t('analytics.title')">
    <h2 class="text-espresso mb-2 text-xs font-semibold tracking-wide uppercase">
      {{ t('analytics.title') }}
    </h2>
    <p v-if="loading && !report" class="text-muted-foreground">{{ t('common.loading') }}</p>
    <p v-else-if="failed && !report" class="text-destructive">{{ t('analytics.failed') }}</p>
    <p v-else-if="!report" class="text-muted-foreground">{{ t('analytics.noData') }}</p>
    <dl v-else class="grid grid-cols-[1fr_auto] gap-x-3 gap-y-1">
      <dt class="text-muted-foreground">{{ t('analytics.llmCalls') }}</dt>
      <dd class="text-right font-medium tabular-nums">{{ n(report.llm.calls, 'int') }}</dd>
      <dt class="text-muted-foreground">{{ t('analytics.llmCost') }}</dt>
      <dd class="text-right font-medium tabular-nums">
        {{ n(Number(report.llm.cost_usd), 'usd') }}
      </dd>
    </dl>
  </section>
</template>
