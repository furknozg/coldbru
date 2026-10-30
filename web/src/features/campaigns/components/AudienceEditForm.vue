<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'

import InlineProblems from '@/components/InlineProblems.vue'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'

import { audienceContent } from '../content'
import type { Problem } from '../problem'
import type { VariantView } from '../types'

/**
 * Edits the rule of the audience (D-070). The server compiles and counts it again and answers with
 * the validation lines when it does not hold; they are shown under the field.
 */
const props = defineProps<{ variant: VariantView; busy: boolean; problem: Problem | null }>()

const emit = defineEmits<{ save: [content: { dsl: string }]; cancel: [] }>()

const { t } = useI18n()
const id = useId()
const dsl = ref(audienceContent(props.variant).dsl)
const valid = computed(() => dsl.value.trim() !== '')
</script>

<template>
  <form class="flex flex-col gap-2" @submit.prevent="valid && emit('save', { dsl: dsl.trim() })">
    <label :for="id" class="text-sm font-medium">{{ t('audience.editLabel') }}</label>
    <Textarea
      :id="id"
      v-model="dsl"
      rows="5"
      spellcheck="false"
      class="font-mono text-xs"
      :aria-invalid="problem !== null"
    />
    <InlineProblems v-if="problem" :problem="problem" />
    <div class="flex justify-end gap-2">
      <Button type="button" variant="outline" :disabled="busy" @click="emit('cancel')">
        {{ t('common.cancel') }}
      </Button>
      <Button type="submit" :disabled="busy || !valid">
        {{ busy ? t('common.sending') : t('common.save') }}
      </Button>
    </div>
  </form>
</template>
