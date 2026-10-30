<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { Badge } from '@/components/ui/badge'

/** The mode the model service runs in; mock is highlighted so nobody mistakes canned text for Claude (D-072). */
defineProps<{ mode: string | null }>()

const { t } = useI18n()
</script>

<template>
  <Badge
    v-if="mode"
    variant="outline"
    :class="mode === 'mock' ? 'bg-primary text-primary-foreground' : 'text-espresso'"
    :title="t('llm.title')"
    :data-mode="mode"
  >
    {{ t('llm.mode', { mode }) }}
  </Badge>
</template>
