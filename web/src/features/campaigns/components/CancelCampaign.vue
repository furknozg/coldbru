<script setup lang="ts">
import { BanIcon } from '@lucide/vue'
import { computed, shallowRef, useId } from 'vue'
import { useI18n } from 'vue-i18n'

import InlineProblems from '@/components/InlineProblems.vue'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'

import { useActions } from '../composables/campaignActions'
import type { Problem } from '../problem'
import { MAX_REASON_CHARS } from '../types'
import ConfirmDialog from './ConfirmDialog.vue'

/**
 * The cancel button of the chat bar (D-091) with its confirmation. The parent shows it only while
 * the campaign can still be cancelled.
 */
const { t, n } = useI18n()
const actions = useActions()
const id = useId()

const cancelling = shallowRef(false)
const reason = shallowRef('')
const cancelProblem = shallowRef<Problem | null>(null)

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
  <Button
    type="button"
    variant="outline"
    class="text-destructive"
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
</template>
