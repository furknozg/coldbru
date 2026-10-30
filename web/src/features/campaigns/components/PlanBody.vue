<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { planContent } from '../content'
import { useLabels } from '../labels'
import type { VariantView } from '../types'

/** The channel plan: how the audience is spread over the channels and how many messages that is. */
const props = defineProps<{ variant: VariantView }>()

const { t, n } = useI18n()
const labels = useLabels()

const plan = computed(() => planContent(props.variant))
</script>

<template>
  <div class="flex flex-col gap-3 text-sm">
    <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1">
      <dt class="text-muted-foreground">{{ t('plan.mode.label') }}</dt>
      <dd class="font-medium">{{ labels.planMode(plan.mode) }}</dd>
      <template v-if="plan.mode !== 'both'">
        <dt class="text-muted-foreground">{{ t('plan.primary') }}</dt>
        <dd class="font-medium">{{ labels.channel(plan.primary) }}</dd>
      </template>
    </dl>
    <p v-if="plan.rationale" class="break-words whitespace-pre-wrap">{{ plan.rationale }}</p>

    <table class="w-full text-left text-xs">
      <caption class="text-muted-foreground pb-1 text-left font-medium">
        {{
          t('plan.recipients')
        }}
      </caption>
      <thead>
        <tr class="text-muted-foreground">
          <th scope="col" class="py-1 pr-2 font-medium">{{ t('audience.language') }}</th>
          <th scope="col" class="py-1 text-right font-medium">{{ t('channel.email') }}</th>
          <th scope="col" class="py-1 pl-2 text-right font-medium">{{ t('channel.sms') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in plan.byLanguage" :key="row.language" class="border-t">
          <th scope="row" class="py-1 pr-2 font-normal">{{ labels.language(row.language) }}</th>
          <td class="py-1 text-right tabular-nums">{{ n(row.email, 'int') }}</td>
          <td class="py-1 pl-2 text-right tabular-nums">{{ n(row.sms, 'int') }}</td>
        </tr>
      </tbody>
      <tfoot>
        <tr class="border-t font-medium">
          <th scope="row" class="py-1 pr-2 text-left">
            {{ t('plan.total', { total: n(plan.recipients.total, 'int') }) }}
          </th>
          <td class="py-1 text-right tabular-nums">{{ n(plan.recipients.email, 'int') }}</td>
          <td class="py-1 pl-2 text-right tabular-nums">{{ n(plan.recipients.sms, 'int') }}</td>
        </tr>
      </tfoot>
    </table>
  </div>
</template>
