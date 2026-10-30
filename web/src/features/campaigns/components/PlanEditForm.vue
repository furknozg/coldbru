<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'

import FormSelect from '@/components/FormSelect.vue'
import InlineProblems from '@/components/InlineProblems.vue'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'

import { CHANNELS, PLAN_MODES, planContent, type PlanMode } from '../content'
import { useLabels } from '../labels'
import type { Problem } from '../problem'
import type { Channel, VariantView } from '../types'

/** Edits the channel plan: the mode, the primary channel and why (D-070). */
const props = defineProps<{ variant: VariantView; busy: boolean; problem: Problem | null }>()

const emit = defineEmits<{
  save: [content: { mode: PlanMode; primary: Channel; rationale: string }]
  cancel: []
}>()

const { t } = useI18n()
const labels = useLabels()
const id = useId()

const initial = planContent(props.variant)
const mode = ref<PlanMode>(initial.mode)
const primary = ref<Channel>(initial.primary)
const rationale = ref(initial.rationale)

const modeOptions = computed(() =>
  PLAN_MODES.map((value) => ({ value, label: labels.planMode(value) })),
)
const primaryOptions = computed(() =>
  CHANNELS.map((value) => ({ value, label: labels.channel(value) })),
)
</script>

<template>
  <form
    class="flex flex-col gap-3"
    @submit.prevent="emit('save', { mode, primary, rationale: rationale.trim() })"
  >
    <div class="flex flex-col gap-1">
      <label :for="`${id}-mode`" class="text-sm font-medium">{{ t('plan.mode.label') }}</label>
      <FormSelect :id="`${id}-mode`" v-model="mode" :options="modeOptions" />
    </div>
    <div class="flex flex-col gap-1">
      <label :for="`${id}-primary`" class="text-sm font-medium">{{ t('plan.primary') }}</label>
      <FormSelect :id="`${id}-primary`" v-model="primary" :options="primaryOptions" />
    </div>
    <div class="flex flex-col gap-1">
      <label :for="`${id}-rationale`" class="text-sm font-medium">{{ t('plan.rationale') }}</label>
      <Textarea :id="`${id}-rationale`" v-model="rationale" rows="4" />
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
