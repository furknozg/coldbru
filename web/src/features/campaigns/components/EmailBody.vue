<script setup lang="ts">
import { LoaderCircleIcon } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'

import { useEmailPreview } from '../composables/useEmailPreview'
import { emailContent } from '../content'
import type { VariantView } from '../types'
import EmailPreviewFrame from './EmailPreviewFrame.vue'

/**
 * An email variant as the recipient sees it (D-090): the stored blocks rendered by the server. While
 * the preview is not there (loading, or the request failed) the blocks are shown as plain text.
 */
const props = defineProps<{ variant: VariantView }>()

const { t } = useI18n()
const email = computed(() => emailContent(props.variant))
const { preview, loading, problem, refresh } = useEmailPreview(
  () => props.variant.id,
  () => undefined,
)
</script>

<template>
  <EmailPreviewFrame
    v-if="preview"
    :subject="preview.subject"
    :html="preview.html"
    :loading="loading"
  />
  <div v-else class="flex flex-col gap-2">
    <p v-if="loading" class="text-muted-foreground flex items-center gap-2 text-xs" role="status">
      <LoaderCircleIcon class="size-3.5 animate-spin" />{{ t('content.previewLoading') }}
    </p>
    <div
      v-else-if="problem"
      role="alert"
      class="text-destructive flex flex-wrap items-center gap-2 text-xs"
    >
      <span>{{ t('content.previewFailed') }}: {{ problem.message }}</span>
      <Button size="xs" variant="outline" @click="refresh()">{{ t('common.retry') }}</Button>
    </div>
    <dl class="flex flex-col gap-2" data-email-blocks>
      <div>
        <dt class="text-muted-foreground text-xs">{{ t('content.subject') }}</dt>
        <dd class="font-medium break-words whitespace-pre-wrap">{{ email.subject }}</dd>
      </div>
      <div>
        <dt class="text-muted-foreground text-xs">{{ t('content.preheader') }}</dt>
        <dd class="break-words whitespace-pre-wrap">{{ email.preheader }}</dd>
      </div>
      <div>
        <dt class="text-muted-foreground text-xs">{{ t('content.headline') }}</dt>
        <dd class="break-words whitespace-pre-wrap">{{ email.headline }}</dd>
      </div>
      <div>
        <dt class="text-muted-foreground text-xs">{{ t('content.paragraphs') }}</dt>
        <dd class="flex flex-col gap-1">
          <span
            v-for="(paragraph, index) in email.paragraphs"
            :key="index"
            class="border-caramel/40 border-l-2 pl-2 break-words whitespace-pre-wrap"
            >{{ paragraph }}</span
          >
        </dd>
      </div>
      <div>
        <dt class="text-muted-foreground text-xs">{{ t('content.cta') }}</dt>
        <dd class="break-words whitespace-pre-wrap">{{ email.ctaLabel }}</dd>
      </div>
    </dl>
  </div>
</template>
