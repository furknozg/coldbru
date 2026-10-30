<script setup lang="ts">
/**
 * Headline figures in a row (D-091, D-097): a small label over a value, no box around each. The
 * value is already formatted; `note` is a muted second line (for example the token split).
 */
defineProps<{ items: readonly { key: string; label: string; value: string; note?: string }[] }>()
</script>

<template>
  <dl class="grid grid-cols-2 gap-x-6 gap-y-4 sm:grid-cols-3 lg:grid-cols-4">
    <div
      v-for="item in items"
      :key="item.key"
      :data-stat="item.key"
      class="flex min-w-0 flex-col gap-0.5"
    >
      <dt class="text-muted-foreground text-xs">{{ item.label }}</dt>
      <dd class="text-espresso text-xl font-semibold tabular-nums break-words">{{ item.value }}</dd>
      <dd v-if="item.note" class="text-muted-foreground text-xs tabular-nums">{{ item.note }}</dd>
    </div>
  </dl>
</template>
