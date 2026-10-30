<script setup lang="ts">
import type { Problem } from '@/features/campaigns/problem'

/** The message of a failed call and its validation lines, next to the form that sent it. */
defineProps<{ problem: Problem }>()
</script>

<template>
  <div
    role="alert"
    class="border-destructive/40 bg-destructive/5 text-destructive rounded-lg border px-3 py-2 text-sm"
  >
    <p class="break-words">{{ problem.message }}</p>
    <ul v-if="problem.problems.length" class="mt-1 list-disc space-y-0.5 pl-5">
      <li v-for="line in problem.problems" :key="line" class="break-words">{{ line }}</li>
    </ul>
  </div>
</template>
