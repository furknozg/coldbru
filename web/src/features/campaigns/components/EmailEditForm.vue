<script setup lang="ts">
import { PlusIcon, Trash2Icon } from '@lucide/vue'
import { computed, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'

import InlineProblems from '@/components/InlineProblems.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

import { useEmailPreview } from '../composables/useEmailPreview'
import { emailContent, PARAGRAPHS_MAX, PARAGRAPHS_MIN, type EmailEdit } from '../content'
import type { Problem } from '../problem'
import type { VariantView } from '../types'
import EmailPreviewFrame from './EmailPreviewFrame.vue'

/**
 * Edits the blocks of an email next to its preview (D-090): every change asks the server for a new
 * render after a short pause, so the marketer sees the result before saving. Save sends the blocks
 * to the edit endpoint; the validation lines of a 422, from the preview or the save, show under the
 * fields.
 */
const props = defineProps<{ variant: VariantView; busy: boolean; problem: Problem | null }>()

const emit = defineEmits<{ save: [content: EmailEdit]; cancel: [] }>()

const { t } = useI18n()
const id = useId()

const initial = emailContent(props.variant)
const subject = ref(initial.subject)
const preheader = ref(initial.preheader)
const headline = ref(initial.headline)
const paragraphs = ref<string[]>(initial.paragraphs.length ? [...initial.paragraphs] : [''])
const ctaLabel = ref(initial.ctaLabel)

const draft = computed<EmailEdit>(() => ({
  subject: subject.value,
  preheader: preheader.value,
  headline: headline.value,
  paragraphs: [...paragraphs.value],
  cta_label: ctaLabel.value,
}))

const live = useEmailPreview(
  () => props.variant.id,
  () => draft.value as unknown as Record<string, unknown>,
)

const canAdd = computed(() => paragraphs.value.length < PARAGRAPHS_MAX)
const canRemove = computed(() => paragraphs.value.length > PARAGRAPHS_MIN)

function addParagraph() {
  if (canAdd.value) paragraphs.value = [...paragraphs.value, '']
}

function removeParagraph(index: number) {
  if (canRemove.value) paragraphs.value = paragraphs.value.filter((_, i) => i !== index)
}

function setParagraph(index: number, value: string | number) {
  paragraphs.value = paragraphs.value.map((p, i) => (i === index ? String(value) : p))
}

function save() {
  emit('save', {
    subject: subject.value.trim(),
    preheader: preheader.value.trim(),
    headline: headline.value.trim(),
    paragraphs: paragraphs.value.map((p) => p.trim()),
    cta_label: ctaLabel.value.trim(),
  })
}
</script>

<template>
  <form
    class="grid gap-4 lg:grid-cols-2"
    :aria-label="t('content.editTitle')"
    @submit.prevent="save"
  >
    <div class="flex flex-col gap-3">
      <div class="flex flex-col gap-1">
        <label :for="`${id}-subject`" class="text-sm font-medium">{{ t('content.subject') }}</label>
        <Input :id="`${id}-subject`" v-model="subject" name="subject" />
      </div>
      <div class="flex flex-col gap-1">
        <label :for="`${id}-preheader`" class="text-sm font-medium">
          {{ t('content.preheader') }}
        </label>
        <Input :id="`${id}-preheader`" v-model="preheader" name="preheader" />
      </div>
      <div class="flex flex-col gap-1">
        <label :for="`${id}-headline`" class="text-sm font-medium">
          {{ t('content.headline') }}
        </label>
        <Input :id="`${id}-headline`" v-model="headline" name="headline" />
      </div>
      <fieldset class="flex flex-col gap-2">
        <legend class="mb-1 text-sm font-medium">{{ t('content.paragraphs') }}</legend>
        <div v-for="(paragraph, index) in paragraphs" :key="index" class="flex items-start gap-1">
          <Textarea
            :model-value="paragraph"
            :aria-label="t('content.paragraph', { n: index + 1 })"
            rows="3"
            name="paragraph"
            @update:model-value="setParagraph(index, $event)"
          />
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            :disabled="!canRemove"
            :aria-label="t('content.removeParagraph', { n: index + 1 })"
            @click="removeParagraph(index)"
          >
            <Trash2Icon />
          </Button>
        </div>
        <div>
          <Button
            type="button"
            variant="outline"
            size="sm"
            :disabled="!canAdd"
            @click="addParagraph"
          >
            <PlusIcon />{{ t('content.addParagraph') }}
          </Button>
        </div>
      </fieldset>
      <div class="flex flex-col gap-1">
        <label :for="`${id}-cta`" class="text-sm font-medium">{{ t('content.cta') }}</label>
        <Input :id="`${id}-cta`" v-model="ctaLabel" name="cta_label" />
      </div>
      <InlineProblems v-if="problem" :problem="problem" />
      <InlineProblems
        v-else-if="live.problem.value && live.problem.value.status === 422"
        :problem="live.problem.value"
      />
      <div class="flex justify-end gap-2">
        <Button type="button" variant="outline" :disabled="busy" @click="emit('cancel')">
          {{ t('common.cancel') }}
        </Button>
        <Button type="submit" :disabled="busy">
          {{ busy ? t('common.sending') : t('common.save') }}
        </Button>
      </div>
    </div>
    <div class="min-w-0">
      <EmailPreviewFrame
        v-if="live.preview.value"
        :subject="live.preview.value.subject"
        :html="live.preview.value.html"
        :loading="live.loading.value"
      />
      <p v-else class="text-muted-foreground text-xs" role="status">
        {{ live.problem.value ? t('content.previewFailed') : t('content.previewLoading') }}
      </p>
    </div>
  </form>
</template>
