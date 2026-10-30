<script setup lang="ts">
import { LoaderCircleIcon, RotateCcwIcon, RefreshCwIcon } from '@lucide/vue'
import { computed, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

import { useActions } from '../composables/campaignActions'
import { useLabels } from '../labels'
import type { CampaignStatus, StepView } from '../types'
import ConfirmDialog from './ConfirmDialog.vue'
import VariantCard from './VariantCard.vue'

/**
 * The step the panel is focused on: its state, the failure and the way out of it, and its variants.
 * An approved step of a drafting campaign can be reopened, which discards it and every later step.
 */
const props = defineProps<{ step: StepView; campaignStatus: CampaignStatus }>()

const { t } = useI18n()
const labels = useLabels()
const actions = useActions()

const reopening = shallowRef(false)

const drafting = computed(() => props.campaignStatus === 'drafting')
const live = computed(() => props.step.variants.filter((v) => v.status !== 'rejected'))
const rejected = computed(() => props.step.variants.filter((v) => v.status === 'rejected'))
const missing = computed(() =>
  (props.step.missing ?? []).map((slot) => labels.slot(slot)).filter(Boolean),
)
const canReopen = computed(() => props.step.status === 'approved' && drafting.value)

async function regenerate() {
  await actions.regenerateStep(props.step.id)
}

async function reopen() {
  const result = await actions.reopen(props.step.kind)
  if (result.ok) reopening.value = false
}
</script>

<template>
  <section class="flex flex-col gap-3" :aria-label="labels.stepKind(step.kind)">
    <header class="flex items-center justify-between gap-2">
      <h3 class="text-base font-semibold">
        {{ labels.stepKind(step.kind) }}
        <span v-if="step.version > 1" class="text-muted-foreground text-xs font-normal">
          {{ t('step.version', { version: step.version }) }}
        </span>
      </h3>
      <Badge variant="outline" :data-step-status="step.status">
        {{ t(`step.status.${step.status}`) }}
      </Badge>
    </header>

    <p
      v-if="step.generating"
      class="text-muted-foreground flex items-center gap-2 text-sm"
      role="status"
    >
      <LoaderCircleIcon class="size-4 animate-spin" />{{ t('step.generating') }}
    </p>

    <div
      v-if="step.failed && !step.generating"
      role="alert"
      class="border-destructive/40 bg-destructive/5 flex flex-col gap-2 rounded-lg border p-3 text-sm"
    >
      <p class="text-destructive font-medium">{{ t('step.failed') }}</p>
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

    <div class="flex flex-col gap-3">
      <VariantCard
        v-for="variant in live"
        :key="variant.id"
        :variant="variant"
        :kind="step.kind"
        :actionable="step.status === 'open' && drafting && variant.status === 'draft'"
      />
    </div>

    <details v-if="rejected.length" class="text-sm">
      <summary class="text-muted-foreground cursor-pointer">
        {{ t('step.rejectedVariants', { count: rejected.length }) }}
      </summary>
      <div class="mt-2 flex flex-col gap-3">
        <VariantCard
          v-for="variant in rejected"
          :key="variant.id"
          :variant="variant"
          :kind="step.kind"
          :actionable="false"
        />
      </div>
    </details>

    <div v-if="canReopen">
      <Button size="sm" variant="outline" :disabled="actions.busy.value" @click="reopening = true">
        <RotateCcwIcon />{{ t('step.reopen') }}
      </Button>
    </div>

    <ConfirmDialog
      v-model:open="reopening"
      :title="t('step.reopenTitle', { step: labels.stepKind(step.kind) })"
      :description="t('step.reopenDescription')"
      :confirm-label="t('step.reopenConfirm')"
      :busy="actions.pending.value === `reopen:${step.kind}`"
      destructive
      @confirm="reopen"
    />
  </section>
</template>
