<script setup lang="ts">
import { LoaderCircleIcon } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

/**
 * In place of a chart whose chunk is still being fetched, or that could not be (D-092): a spinner
 * with its text for assistive technology, or the failure as a plain alert line (D-111). The
 * spinner stops when the reader prefers reduced motion.
 */
defineProps<{ error?: unknown }>()

const { t } = useI18n()
</script>

<template>
  <p v-if="error" class="text-muted-foreground text-xs" role="alert">{{ t('chart.loadFailed') }}</p>
  <p v-else class="text-muted-foreground flex items-center gap-2 text-xs" role="status">
    <LoaderCircleIcon
      class="size-4 motion-safe:animate-spin"
      aria-hidden="true"
      data-slot="chart-spinner"
    />
    <span class="sr-only">{{ t('chart.loading') }}</span>
  </p>
</template>
