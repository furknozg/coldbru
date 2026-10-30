<script setup lang="ts">
import { computed, nextTick, onMounted, useTemplateRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { buildChat, isMessage } from '../chat'
import type { StreamingSlot } from '../composables/useCampaign'
import type { TimelineItem } from '../timeline'
import type { CampaignView } from '../types'
import ChatMessage from './ChatMessage.vue'
import FinalActions from './FinalActions.vue'
import StepBlock from './StepBlock.vue'
import StreamLine from './StreamLine.vue'
import SystemLine from './SystemLine.vue'

/**
 * The chat (D-091): the messages and the campaign's events as plain lines in the order they
 * happened, a block per step (an accordion with its events and actions, and its variants as the
 * only cards) where the step was approved or, for the open step, at the end, and the accordion of
 * the actions on the campaign as a whole last.
 */
const props = defineProps<{
  view: CampaignView
  items: readonly TimelineItem[]
  streaming: readonly StreamingSlot[]
}>()

const { t } = useI18n()
const scroller = useTemplateRef<HTMLElement>('scroller')

const entries = computed(() => buildChat(props.view, props.items))
const openStep = computed(() => props.view.steps.find((s) => s.status === 'open') ?? null)
const drafting = computed(() => props.view.status === 'drafting')
const pendingQuestionId = computed(() =>
  drafting.value ? (openStep.value?.pending_question?.id ?? null) : null,
)

const streamsOf = (stepId: string) => props.streaming.filter((s) => s.stepId === stepId)
/** Text for a step the view does not have yet: a plain line at the end until the refetch lands. */
const orphanStreams = computed(() =>
  props.streaming.filter((s) => !props.view.steps.some((step) => step.id === s.stepId)),
)

/** A reader who scrolled up is left alone; one who is at the end follows the news. */
const NEAR_END_PX = 120

// Opened on a campaign that already has items: start at the end, where the news is.
onMounted(() => {
  const el = scroller.value
  if (el) el.scrollTop = el.scrollHeight
})

watch(
  () =>
    [
      props.items.length + (openStep.value?.variants.length ?? 0),
      props.streaming.map((s) => s.text.length).join(),
    ] as const,
  async ([count], previous) => {
    const el = scroller.value
    if (!el) return
    // The first items arrived after the page opened: start at the end too.
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
    <ol class="mx-auto flex max-w-4xl flex-col gap-4" :aria-label="t('timeline.title')">
      <template v-for="entry in entries" :key="entry.key">
        <StepBlock
          v-if="entry.type === 'step'"
          :step="entry.step"
          :campaign-status="view.status"
          :events="entry.events"
          :streams="streamsOf(entry.step.id)"
          :cost="view.cost"
        />
        <ChatMessage
          v-else-if="isMessage(entry.item)"
          :item="entry.item"
          :pending="entry.item.question?.id === pendingQuestionId && pendingQuestionId !== null"
        />
        <SystemLine v-else :item="entry.item" />
      </template>

      <StreamLine v-for="stream in orphanStreams" :key="stream.key" :stream="stream" />

      <FinalActions :status="view.status" :steps="view.steps" />
    </ol>
  </div>
</template>
