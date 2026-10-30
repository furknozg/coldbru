<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { CostSummary } from '../types'

/** The estimated cost of the campaign's messages, under the content step's variants. */
defineProps<{ cost: CostSummary }>()

const { t, n } = useI18n()
</script>

<template>
  <dl class="text-muted-foreground grid grid-cols-[1fr_auto] gap-x-3 gap-y-0.5 text-xs" data-cost>
    <dt class="col-span-2 font-medium">{{ t('cost.title') }}</dt>
    <dt>{{ t('channel.email') }}</dt>
    <dd class="text-foreground tabular-nums">{{ n(Number(cost.email_usd), 'usd') }}</dd>
    <dt>{{ t('channel.sms') }}</dt>
    <dd class="text-foreground tabular-nums">{{ n(Number(cost.sms_usd), 'usd') }}</dd>
    <dt class="font-medium">{{ t('cost.total') }}</dt>
    <dd class="text-foreground font-medium tabular-nums">{{ n(Number(cost.total_usd), 'usd') }}</dd>
  </dl>
</template>
