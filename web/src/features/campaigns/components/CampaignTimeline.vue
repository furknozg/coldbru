<script setup lang="ts">
import { nextTick, onMounted, useTemplateRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { StreamingSlot } from '../composables/useCampaign'
import type { TimelineItem } from '../timeline'
import StreamingCard from './StreamingCard.vue'
import TimelineCard from './TimelineCard.vue'

/** The centre column: the marketer's messages and the system cards in the order they happened. */
const props = defineProps<{
  items: readonly TimelineItem[]
  streaming: readonly StreamingSlot[]
}>()

const emit = defineEmits<{ 'focus-step': [stepId: string] }>()

const { t } = useI18n()
const scroller = useTemplateRef<HTMLElement>('scroller')

/** A reader who scrolled up is left alone; one who is at the end follows the new cards. */
const NEAR_END_PX = 120

// Opened on a campaign that already has cards: start at the end, where the news is.
onMounted(() => {
  const el = scroller.value
  if (el) el.scrollTop = el.scrollHeight
})

watch(
  () => [props.items.length, props.streaming.map((s) => s.text.length).join()] as const,
  async ([count], previous) => {
    const el = scroller.value
    if (!el) return
    // The first cards arrived after the page opened: start at the end too.
    const opened = (previous?.[0] ?? 0) === 0 && count > 0
    const follow = el.scrollHeight - el.scrollTop - el.clientHeight < NEAR_END_PX
    if (!opened && !follow) return
    await nextTick()
    el.scrollTop = el.scrollHeight
  },
  { flush: 'post' },
)
</script>

<template>
  <div ref="scroller" class="min-h-0 flex-1 overflow-y-auto px-4 py-4">
    <ol class="flex flex-col gap-3" :aria-label="t('timeline.title')">
      <TimelineCard
        v-for="item in items"
        :key="item.key"
        :item="item"
        @focus-step="emit('focus-step', $event)"
      />
      <StreamingCard v-for="stream in streaming" :key="stream.key" :stream="stream" />
    </ol>
  </div>
</template>
