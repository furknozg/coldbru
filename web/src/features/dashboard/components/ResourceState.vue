<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { toProblem } from '@/features/campaigns/problem'

import MetricSkeleton from './MetricSkeleton.vue'

/**
 * The three states every dashboard block shares (D-091, D-111): loading as a skeleton of the block
 * (the `loading` slot, or a row of tiles), failed as a plain line with a retry, and nothing to
 * show as a plain line. The default slot is the block itself: it renders once `ready`, fades in,
 * and stays in place while a refetch is on its way.
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
  <slot v-else-if="loading && !ready" name="loading"><MetricSkeleton /></slot>
  <p v-else-if="ready && empty" class="text-muted-foreground text-sm">{{ empty }}</p>
  <div
    v-else-if="ready"
    class="flex flex-col gap-6 motion-safe:animate-in motion-safe:fade-in motion-safe:duration-300"
    data-slot="resource"
  >
    <slot />
  </div>
</template>
