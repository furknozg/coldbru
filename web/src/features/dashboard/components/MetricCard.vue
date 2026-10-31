<script setup lang="ts">
/**
 * The one surface of the dashboard (D-111): a group of metrics (stat tiles, a chart, a table) in a
 * bordered card with the same inner padding on every side, so a chart's plot lines up with the
 * title and the tiles around it and never touches the edge. The title is the group's heading
 * and every card of the dashboard pages has one; only the skeleton (`MetricSkeleton`), which
 * stands for a card whose title is not known yet, leaves it out. Nothing inside it is a card of
 * its own.
 */
defineProps<{ title?: string }>()
</script>

<template>
  <section
    data-slot="metric-card"
    class="bg-card text-card-foreground flex min-w-0 flex-col gap-4 rounded-xl border p-4"
    :aria-label="title"
  >
    <h3 v-if="title" class="text-espresso text-base font-semibold">{{ title }}</h3>
    <slot />
  </section>
</template>
