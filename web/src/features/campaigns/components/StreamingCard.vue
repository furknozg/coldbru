<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { useLabels } from '../labels'
import type { StreamingSlot } from '../composables/useCampaign'

/** Text the model is writing right now, drawn as it arrives (D-072); replaced by the variant card. */
defineProps<{ stream: StreamingSlot }>()

const { t } = useI18n()
const labels = useLabels()
</script>

<template>
  <li class="flex justify-start" data-kind="streaming">
    <div
      class="border-l-caramel bg-muted/60 max-w-[85%] min-w-0 rounded-xl border border-l-4 px-3 py-2 text-sm"
    >
      <p class="font-medium">
        {{
          t('timeline.streaming', {
            slot: labels.slot(stream.slot) || t('timeline.streamingPlain'),
          })
        }}
        <span v-if="stream.attempt > 1" class="text-muted-foreground text-xs">
          {{ t('timeline.attempt', { n: stream.attempt }) }}
        </span>
      </p>
      <p
        v-if="stream.text"
        class="text-muted-foreground mt-1 max-h-32 overflow-y-auto font-mono text-xs break-words whitespace-pre-wrap"
      >
        {{ stream.text }}
      </p>
    </div>
  </li>
</template>
