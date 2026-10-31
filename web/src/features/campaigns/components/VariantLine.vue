<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { useLabels } from '../labels'
import type { StepKind, VariantView } from '../types'

/**
 * A variant that is not a card (D-091): rejected or replaced, listed in its step's accordion as one
 * line with its state in words. It fades in once, when it is added (D-112).
 */
const props = defineProps<{ variant: VariantView; kind: StepKind }>()

const { t } = useI18n()
const labels = useLabels()

const title = computed(
  () =>
    labels.slot({
      language: props.variant.language,
      channel: props.variant.channel,
      timezone: props.variant.timezone,
    }) || labels.stepKind(props.kind),
)
</script>

<template>
  <li
    class="flex flex-wrap items-baseline gap-x-2 text-xs motion-safe:animate-in motion-safe:fade-in motion-safe:duration-300"
    :data-variant-status="variant.status"
  >
    <span class="break-words">{{ title }}</span>
    <span class="text-muted-foreground">{{ t(`variant.status.${variant.status}`) }}</span>
  </li>
</template>
