<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'

import type { StreamingSlot } from '../composables/useCampaign'
import { useLabels } from '../labels'
import type { TimelineItem } from '../timeline'
import type { CampaignStatus, CostSummary, StepView } from '../types'
import StepCost from './StepCost.vue'
import StepLog from './StepLog.vue'
import VariantCard from './VariantCard.vue'

/**
 * One step in the chat (D-091). One accordion holds the step's events and actions: open while the
 * step is open, closed once it is approved. The variants are the only cards: the drafts of the open
 * step with their actions, and, once the step is approved, only the approved ones. Rejected and
 * replaced variants are listed in the accordion.
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

/** What the reader chose; until then the step's own state decides. */
const chosen = shallowRef<boolean | null>(null)
const expanded = computed(() => chosen.value ?? isOpen.value)
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

const title = computed(() =>
  props.step.version > 1
    ? t('step.withVersion', { step: labels.stepKind(props.step.kind), version: props.step.version })
    : labels.stepKind(props.step.kind),
)
</script>

<template>
  <li
    :id="`step-${step.id}`"
    class="flex min-w-0 flex-col gap-3"
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

    <div v-if="cards.length" class="flex flex-col gap-3">
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
