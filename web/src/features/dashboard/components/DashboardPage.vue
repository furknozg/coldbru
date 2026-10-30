<script setup lang="ts">
/** The frame of a dashboard section: a title, optional controls beside it, and the scrolling body. */
defineProps<{ title: string }>()
</script>

<template>
  <div class="h-full overflow-y-auto">
    <div class="mx-auto flex max-w-5xl flex-col gap-6 px-4 py-6">
      <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <h2 class="text-espresso text-2xl font-semibold">{{ title }}</h2>
        <slot name="actions" />
      </div>
      <slot />
    </div>
  </div>
</template>
