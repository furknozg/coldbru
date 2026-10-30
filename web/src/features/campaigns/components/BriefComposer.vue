<script setup lang="ts">
import { SendIcon } from '@lucide/vue'
import { computed, useId } from 'vue'
import { useI18n } from 'vue-i18n'

import InlineProblems from '@/components/InlineProblems.vue'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'

import type { Problem } from '../problem'

/**
 * The chat bar under the timeline: a brief on the home page, a context note on a drafting campaign.
 * The parent owns the text and clears it when the send worked. The `actions` slot puts more buttons
 * next to the send button (the cancel button of a campaign, D-091).
 */
const props = defineProps<{
  label: string
  placeholder: string
  submitLabel: string
  maxLength: number
  busy?: boolean
  disabled?: boolean
  /** Why the box is disabled, shown instead of the hint. */
  disabledReason?: string
  problem?: Problem | null
}>()

const emit = defineEmits<{ submit: [text: string] }>()

const text = defineModel<string>({ required: true })

const { t, n } = useI18n()
const id = useId()

// The API counts characters, not UTF-16 units.
const length = computed(() => [...text.value].length)
const tooLong = computed(() => length.value > props.maxLength)
const canSubmit = computed(
  () => !props.disabled && !props.busy && text.value.trim() !== '' && !tooLong.value,
)

function submit() {
  if (canSubmit.value) emit('submit', text.value.trim())
}
</script>

<template>
  <form class="flex flex-col gap-2" @submit.prevent="submit">
    <label :for="id" class="sr-only">{{ label }}</label>
    <Textarea
      :id="id"
      v-model="text"
      :placeholder="placeholder"
      :disabled="disabled"
      :aria-invalid="tooLong"
      :aria-describedby="`${id}-help`"
      rows="3"
      class="max-h-48 min-h-20 resize-y"
      @keydown.ctrl.enter.prevent="submit"
      @keydown.meta.enter.prevent="submit"
    />
    <InlineProblems v-if="problem" :problem="problem" />
    <div class="flex items-center justify-between gap-2">
      <p :id="`${id}-help`" class="text-muted-foreground text-xs">
        <template v-if="disabled && disabledReason">{{ disabledReason }}</template>
        <template v-else>{{ t('composer.hint') }}</template>
        <span
          class="ml-2 tabular-nums"
          :class="tooLong ? 'text-destructive font-medium' : ''"
          :aria-label="t('composer.counterLabel')"
        >
          {{ n(length, 'int') }}/{{ n(maxLength, 'int') }}
        </span>
      </p>
      <div class="flex shrink-0 flex-wrap items-center justify-end gap-2">
        <slot name="actions" />
        <Button type="submit" :disabled="!canSubmit">
          <SendIcon />{{ busy ? t('common.sending') : submitLabel }}
        </Button>
      </div>
    </div>
  </form>
</template>
