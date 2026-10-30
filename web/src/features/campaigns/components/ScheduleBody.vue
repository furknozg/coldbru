<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { scheduleContent } from '../content'
import type { VariantView } from '../types'

/** A send time for one (language, time zone) slot, shown in that time zone (D-076); read only here. */
const props = defineProps<{ variant: VariantView }>()

const { t, d } = useI18n()

const schedule = computed(() => scheduleContent(props.variant))

const when = computed(() => {
  const at = schedule.value.sendAt
  if (!at) return t('schedule.immediately')
  try {
    return d(at, { key: 'zoned', timeZone: props.variant.timezone || undefined })
  } catch {
    // An unknown zone name or an unreadable instant makes Intl throw; show what the server sent.
    return at
  }
})
</script>

<template>
  <div class="flex flex-col gap-2 text-sm">
    <p>
      <span class="text-muted-foreground">{{ t('schedule.sendAt') }}: </span>
      <span class="font-medium">{{ when }}</span>
    </p>
    <p v-if="schedule.rationale" class="break-words whitespace-pre-wrap">
      {{ schedule.rationale }}
    </p>
  </div>
</template>
