<script setup lang="ts">
import { computed, nextTick, onMounted, shallowRef, useTemplateRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { StreamingSlot } from '../composables/useCampaign'
import type { TimelineItem } from '../timeline'
import type { CampaignView } from '../types'
import CampaignActions from './CampaignActions.vue'
import QuestionCard from './QuestionCard.vue'
import StepAccordion from './StepAccordion.vue'
import StepDetail from './StepDetail.vue'
import StreamingCard from './StreamingCard.vue'
import TimelineCard from './TimelineCard.vue'

/**
 * The chat (D-089): the cards in the order they happened, with an approved step folded into an
 * accordion on its approval card, then the open step with its variants and actions, the text being
 * written, and the actions on the campaign as a whole. A card about a step brings the step's block
 * into view and unfolds it.
 */
const props = defineProps<{
  view: CampaignView
  items: readonly TimelineItem[]
  streaming: readonly StreamingSlot[]
}>()

const { t } = useI18n()
const scroller = useTemplateRef<HTMLElement>('scroller')

const stepById = computed(() => new Map(props.view.steps.map((s) => [s.id, s])))
const openStep = computed(() => props.view.steps.find((s) => s.status === 'open') ?? null)
const drafting = computed(() => props.view.status === 'drafting')
const pendingQuestionId = computed(() =>
  drafting.value ? (openStep.value?.pending_question?.id ?? null) : null,
)
const scheduleApproved = computed(
  () => props.view.steps.find((s) => s.kind === 'schedule')?.status === 'approved',
)

/** Approved steps the marketer unfolded; the open step is always unfolded. */
const expanded = shallowRef<Readonly<Record<string, boolean>>>({})

function setExpanded(stepId: string, value: boolean) {
  expanded.value = { ...expanded.value, [stepId]: value }
}

async function focusStep(stepId: string) {
  if (stepById.value.get(stepId)?.status === 'approved') setExpanded(stepId, true)
  await nextTick()
  scroller.value?.querySelector(`#step-${CSS.escape(stepId)}`)?.scrollIntoView({ block: 'start' })
}

/** The step an approval card stands for, when the view knows it. */
const approvedStepOf = (item: TimelineItem) =>
  item.kind === 'stepApproved' && item.stepId ? stepById.value.get(item.stepId) : undefined

watch(
  () => props.view.id,
  () => {
    expanded.value = {}
  },
)

/** A reader who scrolled up is left alone; one who is at the end follows the new cards. */
const NEAR_END_PX = 120

// Opened on a campaign that already has cards: start at the end, where the news is.
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
    <ol class="mx-auto flex max-w-4xl flex-col gap-3" :aria-label="t('timeline.title')">
      <template v-for="item in items" :key="item.key">
        <StepAccordion
          v-if="approvedStepOf(item)"
          :step="approvedStepOf(item)!"
          :campaign-status="view.status"
          :cost="view.cost"
          :at="item.at"
          :expanded="expanded[item.stepId!] ?? false"
          @update:expanded="setExpanded(item.stepId!, $event)"
        />
        <QuestionCard
          v-else-if="item.kind === 'question' && item.question"
          :item="item as TimelineItem & { question: NonNullable<TimelineItem['question']> }"
          :pending="item.question.id === pendingQuestionId"
        />
        <TimelineCard v-else :item="item" @focus-step="focusStep" />
      </template>

      <li v-if="openStep" class="flex justify-start" data-kind="openStep">
        <article
          :id="`step-${openStep.id}`"
          class="border-caramel/50 bg-background w-full rounded-xl border p-3 shadow-xs"
          data-testid="open-step"
        >
          <StepDetail :step="openStep" :campaign-status="view.status" :cost="view.cost" />
        </article>
      </li>

      <StreamingCard v-for="stream in streaming" :key="stream.key" :stream="stream" />

      <li class="flex justify-start" data-kind="campaignActions">
        <CampaignActions
          :status="view.status"
          :has-steps="view.steps.length > 0"
          :schedule-approved="scheduleApproved"
        />
      </li>
    </ol>
  </div>
</template>
