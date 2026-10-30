<script setup lang="ts">
import type { Problem } from '@/features/campaigns/problem'

/** The message of a failed call and its validation lines, next to the form that sent it, as plain text: it can sit inside a card (D-091). */
defineProps<{ problem: Problem }>()
</script>

<template>
  <div role="alert" class="text-destructive text-sm">
    <p class="break-words">{{ problem.message }}</p>
    <ul v-if="problem.problems.length" class="mt-1 list-disc space-y-0.5 pl-5">
      <li v-for="line in problem.problems" :key="line" class="break-words">{{ line }}</li>
    </ul>
  </div>
</template>
