<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'

import InlineProblems from '@/components/InlineProblems.vue'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'

import { smsContent } from '../content'
import type { Problem } from '../problem'
import type { VariantView } from '../types'

/**
 * Edits the text of an SMS (D-119): one field, with the `{{first_name}}` placeholder left to the
 * marketer. The count shown is the raw text and only a hint: the server measures the rendered
 * message with the longest first name of the audience and the tracking link (D-028, D-109), and its
 * problems show under the field.
 */
const props = defineProps<{ variant: VariantView; busy: boolean; problem: Problem | null }>()

const emit = defineEmits<{ save: [content: { text: string }]; cancel: [] }>()

const { t, n } = useI18n()
const id = useId()

const text = ref(smsContent(props.variant).text)
const length = computed(() => [...text.value].length)
</script>

<template>
  <form
    class="flex flex-col gap-3"
    :aria-label="t('content.editSmsTitle')"
    @submit.prevent="emit('save', { text: text.trim() })"
  >
    <div class="flex flex-col gap-1">
      <label :for="`${id}-text`" class="text-sm font-medium">{{ t('content.smsText') }}</label>
      <Textarea :id="`${id}-text`" v-model="text" name="text" rows="4" />
      <p class="text-muted-foreground text-xs" data-length>
        {{ t('content.approxLength', { n: n(length, 'int') }) }}
      </p>
    </div>
    <InlineProblems v-if="problem" :problem="problem" />
    <div class="flex justify-end gap-2">
      <Button type="button" variant="outline" :disabled="busy" @click="emit('cancel')">
        {{ t('common.cancel') }}
      </Button>
      <Button type="submit" :disabled="busy">
        {{ busy ? t('common.sending') : t('common.save') }}
      </Button>
    </div>
  </form>
</template>
