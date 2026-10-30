<script setup lang="ts">
import { useI18n } from 'vue-i18n'

/** Plain text in place of a chart that is still being fetched, or that could not be (D-092). */
defineProps<{ error?: unknown }>()

const { t } = useI18n()
</script>

<template>
  <p class="text-muted-foreground text-xs" :role="error ? 'alert' : 'status'">
    {{ error ? t('chart.loadFailed') : t('chart.loading') }}
  </p>
</template>
