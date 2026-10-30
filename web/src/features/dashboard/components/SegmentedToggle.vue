<script setup lang="ts" generic="V extends string">
import { Button } from '@/components/ui/button'

/** Two or three views of the same data, as pressed buttons beside the chart they switch. */
defineProps<{ label: string; options: readonly { value: V; label: string }[] }>()
const model = defineModel<V>({ required: true })
</script>

<template>
  <div class="flex flex-wrap items-center gap-1" role="group" :aria-label="label">
    <Button
      v-for="option in options"
      :key="option.value"
      type="button"
      size="sm"
      variant="ghost"
      :class="model === option.value ? 'bg-accent text-accent-foreground' : 'text-muted-foreground'"
      :aria-pressed="model === option.value"
      @click="model = option.value"
    >
      {{ option.label }}
    </Button>
  </div>
</template>
