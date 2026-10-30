<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { toProblem } from '@/features/campaigns/problem'

/**
 * The three states every dashboard block shares, as plain lines (D-091): loading, failed with a
 * retry, and nothing to show. The default slot is the block itself and renders once `ready`.
 */
const props = defineProps<{
  loading: boolean
  error: unknown
  ready: boolean
  /** The block has loaded but holds nothing; the text says so. */
  empty?: string
}>()
const emit = defineEmits<{ retry: [] }>()

const { t } = useI18n()
const message = computed(() => (props.error ? toProblem(props.error, t).message : ''))
</script>

<template>
  <p v-if="error" role="alert" class="text-destructive text-sm">
    {{ message }}
    <button type="button" class="underline underline-offset-2" @click="emit('retry')">
      {{ t('common.retry') }}
    </button>
  </p>
  <p v-else-if="loading && !ready" role="status" class="text-muted-foreground text-sm">
    {{ t('common.loading') }}
  </p>
  <p v-else-if="ready && empty" class="text-muted-foreground text-sm">{{ empty }}</p>
  <slot v-else-if="ready" />
</template>
