<script setup lang="ts">
import { ChevronsRightIcon, XIcon } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'

import { useLabels } from '../labels'
import { STEP_ORDER, type CampaignView, type StepView } from '../types'
import CampaignActions from './CampaignActions.vue'
import StepDetail from './StepDetail.vue'

/**
 * The right column: the four steps in flow order, the focused step with its variants and actions, the
 * campaign actions and the cost. While a step is open the panel can only be collapsed, because the
 * marketer's decision is waiting in it; with nothing to decide it can be closed (D-087).
 */
const props = defineProps<{
  view: CampaignView
  /** The step the marketer picked; by default the open one. */
  focusedStepId: string | null
}>()

const emit = defineEmits<{
  'focus-step': [stepId: string]
  collapse: []
  close: []
}>()

const { t, n } = useI18n()
const labels = useLabels()

const steps = computed(() => props.view.steps)
const openStep = computed(() => steps.value.find((s) => s.status === 'open') ?? null)
const activeStep = computed<StepView | null>(
  () =>
    steps.value.find((s) => s.id === props.focusedStepId) ??
    openStep.value ??
    steps.value[steps.value.length - 1] ??
    null,
)
const canClose = computed(() => openStep.value === null)

const flow = computed(() =>
  STEP_ORDER.map((kind, index) => ({
    kind,
    index: index + 1,
    step: steps.value.find((s) => s.kind === kind) ?? null,
  })),
)

const scheduleApproved = computed(
  () => steps.value.find((s) => s.kind === 'schedule')?.status === 'approved',
)
const showCost = computed(() => steps.value.some((s) => s.kind === 'content'))
</script>

<template>
  <div class="flex h-full min-h-0 flex-col" data-testid="step-panel">
    <header class="flex items-center justify-between gap-2 border-b px-4 py-3">
      <h2 class="text-sm font-semibold">{{ t('panel.title') }}</h2>
      <Button
        v-if="canClose"
        variant="ghost"
        size="icon-sm"
        :aria-label="t('panel.close')"
        @click="emit('close')"
      >
        <XIcon />
      </Button>
      <Button
        v-else
        variant="ghost"
        size="icon-sm"
        :aria-label="t('panel.collapse')"
        @click="emit('collapse')"
      >
        <ChevronsRightIcon />
      </Button>
    </header>

    <div class="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto p-4">
      <nav :aria-label="t('panel.steps')">
        <ol class="flex flex-col gap-1">
          <li v-for="item in flow" :key="item.kind">
            <button
              type="button"
              class="hover:bg-accent focus-visible:ring-ring/50 flex w-full items-center justify-between gap-2 rounded-lg px-2 py-1.5 text-left text-sm outline-none focus-visible:ring-3 disabled:cursor-default disabled:opacity-60 disabled:hover:bg-transparent"
              :class="
                item.step && item.step.id === activeStep?.id
                  ? 'bg-accent text-accent-foreground'
                  : ''
              "
              :aria-current="item.step && item.step.id === activeStep?.id ? 'step' : undefined"
              :disabled="item.step === null"
              @click="item.step && emit('focus-step', item.step.id)"
            >
              <span>{{ item.index }}. {{ labels.stepKind(item.kind) }}</span>
              <Badge variant="outline" class="text-muted-foreground">
                {{ item.step ? t(`step.status.${item.step.status}`) : t('step.status.waiting') }}
              </Badge>
            </button>
          </li>
        </ol>
      </nav>

      <Separator />

      <StepDetail
        v-if="activeStep"
        :key="activeStep.id"
        :step="activeStep"
        :campaign-status="view.status"
      />
      <p v-else class="text-muted-foreground text-sm">{{ t('panel.noSteps') }}</p>

      <Separator />

      <CampaignActions
        :status="view.status"
        :has-steps="steps.length > 0"
        :schedule-approved="scheduleApproved"
      />

      <dl
        v-if="showCost"
        class="text-muted-foreground grid grid-cols-[1fr_auto] gap-x-3 gap-y-0.5 text-xs"
      >
        <dt class="col-span-2 font-medium">{{ t('cost.title') }}</dt>
        <dt>{{ t('channel.email') }}</dt>
        <dd class="text-foreground tabular-nums">{{ n(Number(view.cost.email_usd), 'usd') }}</dd>
        <dt>{{ t('channel.sms') }}</dt>
        <dd class="text-foreground tabular-nums">{{ n(Number(view.cost.sms_usd), 'usd') }}</dd>
        <dt class="font-medium">{{ t('cost.total') }}</dt>
        <dd class="text-foreground font-medium tabular-nums">
          {{ n(Number(view.cost.total_usd), 'usd') }}
        </dd>
      </dl>
    </div>
  </div>
</template>
