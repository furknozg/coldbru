<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { StreamingSlot } from '../composables/useCampaign'
import { useLabels } from '../labels'

/**
 * Text the model is writing right now, drawn as it arrives (D-072); replaced by the variant card.
 * The line fades in once, when it appears; each delta only changes its text (D-112).
 */
defineProps<{ stream: StreamingSlot }>()

const { t } = useI18n()
const labels = useLabels()
</script>

<template>
  <li
    class="flex min-w-0 flex-col gap-0.5 text-xs motion-safe:animate-in motion-safe:fade-in motion-safe:duration-300"
    data-kind="streaming"
  >
    <p class="font-medium">
      {{
        t('timeline.streaming', {
          slot: labels.slot(stream.slot) || t('timeline.streamingPlain'),
        })
      }}
      <span v-if="stream.attempt > 1" class="text-muted-foreground font-normal">
        {{ t('timeline.attempt', { n: stream.attempt }) }}
      </span>
    </p>
    <p
      v-if="stream.text"
      class="text-muted-foreground max-h-32 overflow-y-auto overscroll-contain font-mono break-words whitespace-pre-wrap"
    >
      {{ stream.text }}
    </p>
  </li>
</template>
