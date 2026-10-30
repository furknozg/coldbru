<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import InlineProblems from '@/components/InlineProblems.vue'
import { Textarea } from '@/components/ui/textarea'

import type { Problem } from '../problem'
import { MAX_REASON_CHARS } from '../types'
import ConfirmDialog from './ConfirmDialog.vue'

/** A rejection needs a reason: the model gets it when the slot is regenerated (D-070). */
const props = defineProps<{
  /** What is rejected, in words: "Türkçe · SMS". */
  target: string
  busy: boolean
  problem: Problem | null
}>()

const emit = defineEmits<{ reject: [reason: string] }>()
const open = defineModel<boolean>('open', { required: true })

const { t, n } = useI18n()
const id = useId()
const reason = ref('')

const length = computed(() => [...reason.value].length)
const valid = computed(() => reason.value.trim() !== '' && length.value <= MAX_REASON_CHARS)

// A reopened dialog starts empty.
watch(open, (isOpen) => {
  if (isOpen) reason.value = ''
})

function confirm() {
  if (valid.value && !props.busy) emit('reject', reason.value.trim())
}
</script>

<template>
  <ConfirmDialog
    v-model:open="open"
    :title="t('reject.title')"
    :description="t('reject.description', { target })"
    :confirm-label="t('reject.confirm')"
    :busy="busy"
    :can-confirm="valid"
    destructive
    @confirm="confirm"
  >
    <div class="flex flex-col gap-2">
      <label :for="id" class="text-sm font-medium">{{ t('reject.reasonLabel') }}</label>
      <Textarea
        :id="id"
        v-model="reason"
        rows="4"
        required
        :aria-invalid="length > MAX_REASON_CHARS"
        :placeholder="t('reject.reasonPlaceholder')"
      />
      <p
        class="text-xs tabular-nums"
        :class="length > MAX_REASON_CHARS ? 'text-destructive' : 'text-muted-foreground'"
      >
        {{ n(length, 'int') }}/{{ n(MAX_REASON_CHARS, 'int') }}
      </p>
      <InlineProblems v-if="problem" :problem="problem" />
    </div>
  </ConfirmDialog>
</template>
