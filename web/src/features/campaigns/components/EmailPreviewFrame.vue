<script setup lang="ts">
import { useI18n } from 'vue-i18n'

/**
 * The rendered email (D-090): the subject with the sample name in it and the HTML document in an
 * iframe. The `sandbox` attribute carries no token, so the document runs no script, has a unique
 * origin, cannot navigate this page and cannot submit forms; its inline CSS stays inside the frame.
 * Without `allow-same-origin` the page cannot read the document to measure it, so instead of a
 * height computed from the content the frame has a fixed height the marketer can drag (CSS
 * `resize`); isolation was preferred over an exact fit.
 */
defineProps<{ subject: string; html: string; loading?: boolean }>()

const { t } = useI18n()
</script>

<template>
  <div class="flex flex-col gap-1" data-email-preview>
    <p class="text-muted-foreground text-xs">
      {{ t('content.subject') }} · {{ t('content.previewSample') }}
    </p>
    <p class="font-medium break-words" data-preview-subject>{{ subject }}</p>
    <div
      class="h-[28rem] min-h-40 resize-y overflow-hidden"
      :class="loading ? 'opacity-70' : ''"
      :aria-busy="loading || undefined"
    >
      <iframe
        sandbox=""
        :srcdoc="html"
        :title="t('content.previewFrameTitle')"
        class="h-full w-full border-0 bg-white"
        loading="lazy"
      />
    </div>
  </div>
</template>
