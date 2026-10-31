<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { TimelineItem } from '../timeline'

/**
 * An event of the campaign that belongs to no step (screening, sending, cancelling), or to a step
 * the view does not know yet: one muted line, no box (D-091). It fades in once, when it is added
 * (D-112).
 */
const { item } = defineProps<{ item: TimelineItem }>()

const { d } = useI18n()

const tones: Record<TimelineItem['tone'], string> = {
  neutral: 'text-muted-foreground',
  success: 'text-foreground',
  warning: 'text-caramel',
  danger: 'text-destructive',
}
</script>

<template>
  <li
    class="flex min-w-0 flex-col text-xs motion-safe:animate-in motion-safe:fade-in motion-safe:duration-300"
    :class="tones[item.tone]"
    :data-kind="item.kind"
  >
    <p class="flex items-baseline gap-2">
      <span class="font-medium">{{ item.title }}</span>
      <time class="text-muted-foreground shrink-0" :datetime="item.at">
        {{ d(item.at, 'time') }}
      </time>
    </p>
    <p v-if="item.body" class="text-muted-foreground break-words whitespace-pre-wrap">
      {{ item.body }}
    </p>
  </li>
</template>
