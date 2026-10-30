<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { TimelineItem } from '../timeline'

const { item } = defineProps<{ item: TimelineItem }>()
const emit = defineEmits<{ 'focus-step': [stepId: string] }>()

const { t, d } = useI18n()

const tones: Record<TimelineItem['tone'], string> = {
  neutral: 'border-l-border',
  success: 'border-l-sage',
  warning: 'border-l-caramel',
  danger: 'border-l-destructive',
}

const focusable = computed(() => item.author === 'system' && Boolean(item.stepId))
</script>

<template>
  <li
    class="flex"
    :class="item.author === 'user' ? 'justify-end' : 'justify-start'"
    :data-kind="item.kind"
  >
    <component
      :is="focusable ? 'button' : 'div'"
      :type="focusable ? 'button' : undefined"
      :aria-label="focusable ? t('timeline.showStep', { title: item.title }) : undefined"
      class="max-w-[85%] min-w-0 rounded-xl border px-3 py-2 text-left text-sm"
      :class="[
        item.author === 'user'
          ? 'border-caramel/40 bg-background'
          : `border-l-4 bg-muted/60 ${tones[item.tone]}`,
        focusable
          ? 'hover:bg-accent focus-visible:ring-ring/50 cursor-pointer outline-none focus-visible:ring-3'
          : '',
      ]"
      @click="focusable && item.stepId && emit('focus-step', item.stepId)"
    >
      <span class="flex items-baseline justify-between gap-3">
        <span class="font-medium" :class="item.author === 'user' ? 'text-espresso' : ''">
          {{ item.title }}
        </span>
        <time class="text-muted-foreground shrink-0 text-xs" :datetime="item.at">
          {{ d(item.at, 'time') }}
        </time>
      </span>
      <span v-if="item.body" class="mt-1 block break-words whitespace-pre-wrap">{{
        item.body
      }}</span>
    </component>
  </li>
</template>
