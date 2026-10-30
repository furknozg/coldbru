<script setup lang="ts">
import { BanIcon, RefreshCwIcon, SendIcon } from '@lucide/vue'
import { computed, shallowRef, useId } from 'vue'
import { useI18n } from 'vue-i18n'

import InlineProblems from '@/components/InlineProblems.vue'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'

import { useActions } from '../composables/campaignActions'
import type { Problem } from '../problem'
import { CANCELLABLE, MAX_REASON_CHARS, type CampaignStatus } from '../types'
import ConfirmDialog from './ConfirmDialog.vue'

/**
 * What can be done to the campaign as a whole: cancel it, retry a confirmation that failed, restart
 * a start that never produced a step.
 */
const props = defineProps<{
  status: CampaignStatus
  hasSteps: boolean
  /** The schedule step is approved, so the campaign should already be scheduled. */
  scheduleApproved: boolean
}>()

const { t, n } = useI18n()
const actions = useActions()
const id = useId()

const cancelling = shallowRef(false)
const reason = shallowRef('')
const cancelProblem = shallowRef<Problem | null>(null)

const drafting = computed(() => props.status === 'drafting')
const canCancel = computed(() => CANCELLABLE.includes(props.status))
const canRetryConfirm = computed(() => drafting.value && props.scheduleApproved)
const canRestart = computed(() => drafting.value && !props.hasSteps)
const reasonLength = computed(() => [...reason.value].length)
const reasonOk = computed(() => reasonLength.value <= MAX_REASON_CHARS)

function openCancel() {
  reason.value = ''
  cancelProblem.value = null
  cancelling.value = true
}

async function cancel() {
  const trimmed = reason.value.trim()
  const result = await actions.cancel(trimmed === '' ? undefined : trimmed)
  if (result.ok) cancelling.value = false
  else cancelProblem.value = result.problem.status === 422 ? result.problem : null
}
</script>

<template>
  <div v-if="canCancel || canRetryConfirm || canRestart" class="flex flex-col gap-2">
    <div v-if="canRestart" class="flex flex-col gap-2 text-sm">
      <p class="text-muted-foreground">{{ t('campaign.noSteps') }}</p>
      <Button
        size="sm"
        variant="outline"
        :disabled="actions.busy.value"
        @click="actions.regenerateCampaign()"
      >
        <RefreshCwIcon />
        {{
          actions.pending.value === 'regenerate-campaign'
            ? t('common.sending')
            : t('campaign.restart')
        }}
      </Button>
    </div>

    <div v-if="canRetryConfirm" class="flex flex-col gap-2 text-sm">
      <p class="text-muted-foreground">{{ t('campaign.confirmPending') }}</p>
      <Button size="sm" :disabled="actions.busy.value" @click="actions.confirm()">
        <SendIcon />
        {{ actions.pending.value === 'confirm' ? t('common.sending') : t('campaign.retryConfirm') }}
      </Button>
    </div>

    <Button
      v-if="canCancel"
      size="sm"
      variant="destructive"
      :disabled="actions.busy.value"
      @click="openCancel"
    >
      <BanIcon />{{ t('campaign.cancel') }}
    </Button>

    <ConfirmDialog
      v-model:open="cancelling"
      :title="t('campaign.cancelTitle')"
      :description="t('campaign.cancelDescription')"
      :confirm-label="t('campaign.cancelConfirm')"
      :busy="actions.pending.value === 'cancel'"
      :can-confirm="reasonOk"
      destructive
      @confirm="cancel"
    >
      <div class="flex flex-col gap-2">
        <label :for="id" class="text-sm font-medium">{{ t('campaign.cancelReasonLabel') }}</label>
        <Textarea :id="id" v-model="reason" rows="3" :aria-invalid="!reasonOk" />
        <p
          class="text-xs tabular-nums"
          :class="reasonOk ? 'text-muted-foreground' : 'text-destructive'"
        >
          {{ n(reasonLength, 'int') }}/{{ n(MAX_REASON_CHARS, 'int') }}
        </p>
        <InlineProblems v-if="cancelProblem" :problem="cancelProblem" />
      </div>
    </ConfirmDialog>
  </div>
</template>
