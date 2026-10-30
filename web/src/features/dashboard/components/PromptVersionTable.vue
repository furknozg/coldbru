<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { PromptVersionUsage } from '@/api/analytics/service'

import { toNumber } from '../series'

/**
 * Every prompt name and version with the outcomes of its calls (D-060, D-083): how many were
 * accepted, repaired or dropped, the repair rate, the mean latency and the cost.
 */
defineProps<{ rows: readonly PromptVersionUsage[] }>()

const { t, n } = useI18n()
</script>

<template>
  <div class="overflow-x-auto">
    <table class="w-full text-sm">
      <caption class="sr-only">
        {{
          t('dashboard.llm.versions')
        }}
      </caption>
      <thead>
        <tr class="text-muted-foreground border-b text-left text-xs">
          <th scope="col" class="py-1.5 pr-3 font-medium">{{ t('dashboard.llm.prompt') }}</th>
          <th scope="col" class="px-3 py-1.5 text-right font-medium">
            {{ t('dashboard.llm.calls') }}
          </th>
          <th scope="col" class="px-3 py-1.5 text-right font-medium">
            {{ t('dashboard.llm.accepted') }}
          </th>
          <th scope="col" class="px-3 py-1.5 text-right font-medium">
            {{ t('dashboard.llm.validationFailed') }}
          </th>
          <th scope="col" class="px-3 py-1.5 text-right font-medium">
            {{ t('dashboard.llm.refusal') }}
          </th>
          <th scope="col" class="px-3 py-1.5 text-right font-medium">
            {{ t('dashboard.llm.apiError') }}
          </th>
          <th scope="col" class="px-3 py-1.5 text-right font-medium">
            {{ t('dashboard.llm.repairRate') }}
          </th>
          <th scope="col" class="px-3 py-1.5 text-right font-medium">
            {{ t('dashboard.llm.latency') }}
          </th>
          <th scope="col" class="py-1.5 pl-3 text-right font-medium">
            {{ t('dashboard.llm.cost') }}
          </th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="row in rows"
          :key="`${row.prompt_name}@${row.prompt_version}`"
          class="border-b last:border-b-0"
          data-row="prompt-version"
        >
          <th scope="row" class="py-1.5 pr-3 text-left font-medium">
            {{ row.prompt_name }}
            <span class="text-muted-foreground font-normal">{{ row.prompt_version }}</span>
          </th>
          <td class="px-3 py-1.5 text-right tabular-nums">{{ n(row.calls, 'int') }}</td>
          <td class="px-3 py-1.5 text-right tabular-nums">{{ n(row.accepted, 'int') }}</td>
          <td class="px-3 py-1.5 text-right tabular-nums">{{ n(row.validation_failed, 'int') }}</td>
          <td class="px-3 py-1.5 text-right tabular-nums">{{ n(row.refusal, 'int') }}</td>
          <td class="px-3 py-1.5 text-right tabular-nums">{{ n(row.api_error, 'int') }}</td>
          <td class="px-3 py-1.5 text-right tabular-nums">{{ n(row.repair_rate, 'percent') }}</td>
          <td class="px-3 py-1.5 text-right tabular-nums">
            {{ t('dashboard.llm.ms', { value: n(row.avg_latency_ms, 'int') }) }}
          </td>
          <td class="py-1.5 pl-3 text-right tabular-nums">
            {{ n(toNumber(row.cost_usd), 'usd') }}
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
