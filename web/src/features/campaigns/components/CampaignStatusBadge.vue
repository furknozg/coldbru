<script setup lang="ts">
import { computed } from 'vue'

import { Badge } from '@/components/ui/badge'

import { useLabels } from '../labels'
import type { CampaignStatus } from '../types'

const { status } = defineProps<{ status: CampaignStatus }>()

const labels = useLabels()

// Palette of D-086: caramel for what is moving, sage for what is done, espresso text on white for a
// draft; cream is kept for hover and selected states.
const styles: Record<CampaignStatus, string> = {
  drafting: 'border-caramel/50 bg-background text-espresso',
  scheduled: 'bg-sage/25 text-espresso',
  sending: 'bg-primary text-primary-foreground',
  completed: 'bg-sage text-ink',
  cancelled: 'border-border bg-muted text-muted-foreground',
}

const label = computed(() => labels.status(status))
</script>

<template>
  <Badge variant="outline" :class="styles[status]" :data-status="status">{{ label }}</Badge>
</template>
