<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Badge } from '@/components/ui/badge'

import { useLabels } from '../labels'
import type { CampaignStatus, CostSummary, StepView } from '../types'
import StepDetail from './StepDetail.vue'

/**
 * An approved step in the chat (D-089): one accordion item on the card that says the step was
 * approved. Closed it is a single line; open it shows the variants, read only, and the reopen action.
 */
const props = defineProps<{
  step: StepView
  campaignStatus: CampaignStatus
  cost?: CostSummary
  at: string
  expanded: boolean
}>()

const emit = defineEmits<{ 'update:expanded': [value: boolean] }>()

const { t, d } = useI18n()
const labels = useLabels()

const title = computed(() =>
  props.step.version > 1
    ? t('step.withVersion', { step: labels.stepKind(props.step.kind), version: props.step.version })
    : labels.stepKind(props.step.kind),
)
const model = computed(() => (props.expanded ? props.step.id : ''))
</script>

<template>
  <li class="flex justify-start" data-kind="stepApproved" :data-step-id="step.id">
    <Accordion
      :id="`step-${step.id}`"
      type="single"
      collapsible
      class="border-l-sage bg-muted/60 w-full max-w-[85%] rounded-xl border border-l-4 px-3 py-1"
      :model-value="model"
      @update:model-value="emit('update:expanded', $event === step.id)"
    >
      <AccordionItem :value="step.id" class="border-b-0">
        <AccordionTrigger class="items-center gap-3 py-1.5">
          <span class="flex min-w-0 flex-1 flex-wrap items-center gap-2">
            <span>{{ t('timeline.stepApproved', { step: title }) }}</span>
            <Badge variant="outline" :data-step-status="step.status">
              {{ t(`step.status.${step.status}`) }}
            </Badge>
          </span>
          <time class="text-muted-foreground shrink-0 text-xs font-normal" :datetime="at">
            {{ d(at, 'time') }}
          </time>
        </AccordionTrigger>
        <AccordionContent class="pt-2">
          <StepDetail :step="step" :campaign-status="campaignStatus" :cost="cost" headless />
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  </li>
</template>
