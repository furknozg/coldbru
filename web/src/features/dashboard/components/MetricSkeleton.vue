<script setup lang="ts">
import { LoaderCircleIcon } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import { Skeleton } from '@/components/ui/skeleton'

import MetricCard from './MetricCard.vue'

/**
 * What a dashboard block looks like before its data is there (D-111): the cards it will fill, with
 * a bar where its title goes, a skeleton tile per figure and a spinner where each chart or table
 * will be, instead of a line of text. It is one status region with a label for assistive
 * technology; the motion stops when the reader prefers reduced motion.
 */
withDefaults(defineProps<{ tiles?: number; charts?: number; rows?: number }>(), {
  tiles: 4,
  charts: 0,
  rows: 0,
})

const { t } = useI18n()
</script>

<template>
  <div class="flex flex-col gap-6" role="status" data-testid="metric-skeleton">
    <span class="sr-only">{{ t('common.loading') }}</span>
    <MetricCard v-if="tiles > 0" aria-hidden="true">
      <Skeleton class="h-4 w-40" />
      <div class="grid grid-cols-2 gap-x-6 gap-y-4 sm:grid-cols-3 lg:grid-cols-4">
        <div v-for="k in tiles" :key="k" class="flex flex-col gap-1.5">
          <Skeleton class="h-3 w-20" />
          <Skeleton class="h-6 w-24" />
        </div>
      </div>
    </MetricCard>
    <MetricCard v-for="k in charts" :key="`chart-${k}`" aria-hidden="true">
      <Skeleton class="h-4 w-40" />
      <div class="flex h-52 w-full items-center justify-center" data-slot="chart-spinner">
        <LoaderCircleIcon class="text-muted-foreground size-6 motion-safe:animate-spin" />
      </div>
    </MetricCard>
    <MetricCard v-if="rows > 0" aria-hidden="true">
      <Skeleton class="h-4 w-40" />
      <div class="flex flex-col gap-2">
        <Skeleton v-for="k in rows" :key="k" class="h-6 w-full" />
      </div>
    </MetricCard>
  </div>
</template>
