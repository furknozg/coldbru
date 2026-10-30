<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { LazyDistributionPie } from '@/components/LazyDistributionPie'

import { CHANNELS, planContent } from '../content'
import { CHANNEL_COLORS } from '../distribution'
import { useLabels } from '../labels'
import type { VariantView } from '../types'

/** The channel plan: how the audience is spread over the channels and how many messages that is. */
const props = defineProps<{ variant: VariantView }>()

const { t } = useI18n()
const labels = useLabels()

const plan = computed(() => planContent(props.variant))

const recipients = computed(() =>
  CHANNELS.map((channel) => ({
    key: channel,
    label: labels.channel(channel),
    value: plan.value.recipients[channel],
    color: CHANNEL_COLORS[channel],
  })),
)
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

    <LazyDistributionPie :title="t('plan.recipients')" :slices="recipients" />
  </div>
</template>
