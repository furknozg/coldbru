<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'

import { carouselGroups } from '../carousel'
import type { StreamingSlot } from '../composables/useCampaign'
import { useLabels } from '../labels'
import type { TimelineItem } from '../timeline'
import type { CampaignStatus, CostSummary, StepView } from '../types'
import StepCost from './StepCost.vue'
import StepLog from './StepLog.vue'
import VariantCard from './VariantCard.vue'
import VariantCarousel from './VariantCarousel.vue'

/**
 * One step in the chat (D-091). One accordion holds the step's events and actions. It starts
 * folded whatever the step's state (D-112); once the reader unfolds or folds it, that choice holds
 * until the step's status changes. The variants are the only cards: the drafts of the open
 * step with their actions, and, once the step is approved, only the approved ones. Rejected and
 * replaced variants are listed in the accordion. The text step's cards sit in one carousel per
 * channel and the schedule step's in one (D-095), each with a count of the slots still waiting for
 * an approval while the step is open. The block fades in once, when it is added to the chat
 * (D-112); later updates of the step do not replay it.
 */
const props = defineProps<{
  step: StepView
  campaignStatus: CampaignStatus
  events: readonly TimelineItem[]
  streams: readonly StreamingSlot[]
  cost?: CostSummary
}>()

const { t } = useI18n()
const labels = useLabels()

const drafting = computed(() => props.campaignStatus === 'drafting')
const isOpen = computed(() => props.step.status === 'open')

/** What the reader chose; until then the accordion is folded (D-112). */
const chosen = shallowRef<boolean | null>(null)
const expanded = computed(() => chosen.value ?? false)
watch(
  () => props.step.status,
  () => {
    chosen.value = null
  },
)

const cardsShown = computed(() => isOpen.value || props.step.status === 'approved')
const cards = computed(() =>
  cardsShown.value
    ? props.step.variants.filter((v) => v.status === 'draft' || v.status === 'approved')
    : [],
)
const listed = computed(() => props.step.variants.filter((v) => !cards.value.includes(v)))

// The text and schedule steps show their cards in carousels (D-095): the text step one per channel.
const carousels = computed(() =>
  props.step.kind === 'content' || props.step.kind === 'schedule'
    ? carouselGroups(props.step, cards.value, isOpen.value)
    : [],
)

const title = computed(() =>
  props.step.version > 1
    ? t('step.withVersion', { step: labels.stepKind(props.step.kind), version: props.step.version })
    : labels.stepKind(props.step.kind),
)
</script>

<template>
  <li
    :id="`step-${step.id}`"
    class="flex min-w-0 flex-col gap-3 motion-safe:animate-in motion-safe:fade-in motion-safe:duration-300"
    data-kind="step"
    :data-step-id="step.id"
    :data-step-status="step.status"
    :data-testid="isOpen ? 'open-step' : undefined"
  >
    <Accordion
      type="single"
      collapsible
      :model-value="expanded ? step.id : ''"
      @update:model-value="chosen = $event === step.id"
    >
      <AccordionItem :value="step.id" class="border-b-0">
        <AccordionTrigger class="items-center justify-start gap-2 py-1 text-xs">
          <span>{{ title }}</span>
          <span class="text-muted-foreground font-normal" data-step-state>
            {{ t(`step.status.${step.status}`) }}
          </span>
        </AccordionTrigger>
        <AccordionContent>
          <StepLog
            :step="step"
            :campaign-status="campaignStatus"
            :events="events"
            :streams="streams"
            :listed="listed"
          />
        </AccordionContent>
      </AccordionItem>
    </Accordion>

    <template v-if="carousels.length">
      <VariantCarousel
        v-for="group in carousels"
        :key="group.key"
        :data-carousel="group.key"
        :variants="group.variants"
        :kind="step.kind"
        :actionable="isOpen && drafting"
        :pending="group.pending"
        :label="group.channel ? labels.channel(group.channel) : undefined"
      />
    </template>
    <div v-else-if="cards.length" class="flex flex-col gap-3">
      <VariantCard
        v-for="variant in cards"
        :key="variant.id"
        :variant="variant"
        :kind="step.kind"
        :actionable="isOpen && drafting && variant.status === 'draft'"
      />
    </div>

    <StepCost v-if="cost && step.kind === 'content' && cardsShown" :cost="cost" />
  </li>
</template>
