<script setup lang="ts" generic="T extends string">
import { cn } from 'cn'
import type { HTMLAttributes } from 'vue'

/** A native `<select>`, styled like the shadcn inputs; the keyboard and screen readers get it free. */
defineProps<{
  options: readonly { value: T; label: string }[]
  class?: HTMLAttributes['class']
}>()

const model = defineModel<T>({ required: true })
</script>

<template>
  <select
    v-model="model"
    :class="
      cn(
        'border-input bg-background focus-visible:border-ring focus-visible:ring-ring/50 h-8 w-full rounded-lg border px-2 text-sm outline-none focus-visible:ring-3 disabled:opacity-50',
        $props.class,
      )
    "
  >
    <option v-for="option in options" :key="option.value" :value="option.value">
      {{ option.label }}
    </option>
  </select>
</template>
