<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { Badge } from '@/components/ui/badge'

/** The round count at the corner of a carousel's card area: slots without an approved variant. */
defineProps<{ count: number; label?: string }>()

const { t } = useI18n()
</script>

<template>
  <Badge
    v-if="count > 0"
    class="bg-primary text-primary-foreground absolute -top-2 -right-2 size-6 justify-center rounded-full px-0 tabular-nums"
    role="status"
    :aria-label="
      label ? t('carousel.pendingFor', { name: label, count }) : t('carousel.pending', { count })
    "
    data-pending
  >
    {{ count }}
  </Badge>
</template>
