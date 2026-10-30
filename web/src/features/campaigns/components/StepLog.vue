<script setup lang="ts">
import { LoaderCircleIcon, RefreshCwIcon, RotateCcwIcon } from '@lucide/vue'
import { computed, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'

import { useActions } from '../composables/campaignActions'
import type { StreamingSlot } from '../composables/useCampaign'
import { useLabels } from '../labels'
import type { TimelineItem } from '../timeline'
import type { CampaignStatus, StepView, VariantView } from '../types'
import ConfirmDialog from './ConfirmDialog.vue'
import StreamLine from './StreamLine.vue'
import VariantLine from './VariantLine.vue'

/**
 * The inside of a step's accordion (D-091), without any box: what happened to the step (opened,
 * generating, failed, approved), the text being written, the way out of a failure, the variants
 * that are not cards (rejected, replaced) and, for an approved step of a drafting campaign, the
 * reopen action (which discards the step and every later one).
 */
const props = defineProps<{
  step: StepView
  campaignStatus: CampaignStatus
  events: readonly TimelineItem[]
  streams: readonly StreamingSlot[]
  /** The variants the step shows as cards are not repeated here. */
  listed: readonly VariantView[]
}>()

const { t, d } = useI18n()
const labels = useLabels()
const actions = useActions()

const missing = computed(() =>
  (props.step.missing ?? []).map((slot) => labels.slot(slot)).filter(Boolean),
)
const failed = computed(() => props.step.failed && !props.step.generating)

const reopening = shallowRef(false)
const canReopen = computed(
  () => props.step.status === 'approved' && props.campaignStatus === 'drafting',
)

async function reopen() {
  const result = await actions.reopen(props.step.kind)
  if (result.ok) reopening.value = false
}

async function regenerate() {
  await actions.regenerateStep(props.step.id)
}
</script>

<template>
  <div class="flex flex-col gap-2" :data-testid="`step-log-${step.id}`">
    <ol v-if="events.length || streams.length" class="flex flex-col gap-1">
      <li
        v-for="event in events"
        :key="event.key"
        class="flex flex-col text-xs"
        :class="event.tone === 'danger' ? 'text-destructive' : 'text-muted-foreground'"
        :data-kind="event.kind"
      >
        <p class="flex items-baseline gap-2">
          <span>{{ event.title }}</span>
          <time class="shrink-0" :datetime="event.at">{{ d(event.at, 'time') }}</time>
        </p>
        <p v-if="event.body" class="break-words whitespace-pre-wrap">{{ event.body }}</p>
      </li>
      <StreamLine v-for="stream in streams" :key="stream.key" :stream="stream" />
    </ol>

    <p
      v-if="step.generating"
      class="text-muted-foreground flex items-center gap-2 text-sm"
      role="status"
    >
      <LoaderCircleIcon class="size-4 animate-spin" />{{ t('step.generating') }}
    </p>

    <div v-if="failed" role="alert" class="text-destructive flex flex-col gap-2 text-sm">
      <p class="font-medium">{{ t('step.failed') }}</p>
      <p v-if="step.failure_reason" class="break-words whitespace-pre-wrap">
        {{ step.failure_reason }}
      </p>
      <div>
        <Button size="sm" variant="outline" :disabled="actions.busy.value" @click="regenerate">
          <RefreshCwIcon />
          {{
            actions.pending.value === `regenerate:${step.id}`
              ? t('common.sending')
              : t('step.regenerate')
          }}
        </Button>
      </div>
    </div>

    <p v-if="missing.length && step.status === 'open'" class="text-muted-foreground text-sm">
      {{ t('step.missing', { slots: missing.join(', ') }) }}
    </p>

    <div v-if="listed.length" class="flex flex-col gap-1">
      <p class="text-muted-foreground text-xs font-medium">{{ t('step.otherVariants') }}</p>
      <ul class="flex flex-col gap-0.5">
        <VariantLine
          v-for="variant in listed"
          :key="variant.id"
          :variant="variant"
          :kind="step.kind"
        />
      </ul>
    </div>

    <div v-if="canReopen">
      <Button size="sm" variant="outline" :disabled="actions.busy.value" @click="reopening = true">
        <RotateCcwIcon />{{ t('step.reopen') }}
      </Button>
    </div>

    <ConfirmDialog
      v-if="canReopen"
      v-model:open="reopening"
      :title="t('step.reopenTitle', { step: labels.stepKind(step.kind) })"
      :description="t('step.reopenDescription')"
      :confirm-label="t('step.reopenConfirm')"
      :busy="actions.pending.value === `reopen:${step.kind}`"
      destructive
      @confirm="reopen"
    />
  </div>
</template>
