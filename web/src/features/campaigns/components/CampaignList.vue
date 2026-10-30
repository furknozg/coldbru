<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { toProblem } from '../problem'
import type { CampaignInfo } from '../types'
import CampaignStatusBadge from './CampaignStatusBadge.vue'

/** The loaded campaigns, newest first: status, title (or the brief) and time (D-089). */
const props = defineProps<{
  items: readonly CampaignInfo[]
  selectedId: string | null
  loading: boolean
  loadingMore: boolean
  hasMore: boolean
  error: unknown
}>()

const emit = defineEmits<{
  navigate: []
  'load-more': []
  retry: []
}>()

const { t, d } = useI18n()

const errorMessage = computed(() => (props.error ? toProblem(props.error, t).message : ''))
</script>

<template>
  <section class="flex min-h-0 flex-1 flex-col gap-2" :aria-label="t('list.title')">
    <div v-if="loading" class="flex flex-col gap-2" role="status" :aria-label="t('common.loading')">
      <Skeleton v-for="n in 4" :key="n" class="h-16 w-full" />
    </div>

    <div
      v-else-if="error && items.length === 0"
      role="alert"
      class="text-destructive space-y-2 text-sm"
    >
      <p>{{ errorMessage }}</p>
      <Button variant="outline" size="sm" @click="emit('retry')">{{ t('common.retry') }}</Button>
    </div>

    <p v-else-if="items.length === 0" class="text-muted-foreground text-sm">
      {{ t('list.empty') }}
    </p>

    <ul v-else class="-mx-1 flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto px-1 pb-1">
      <li v-for="campaign in items" :key="campaign.id">
        <RouterLink
          :to="{ name: 'campaign', params: { id: campaign.id } }"
          class="hover:bg-accent focus-visible:ring-ring/50 flex flex-col gap-1 rounded-lg border p-2 text-sm outline-none focus-visible:ring-3"
          :class="campaign.id === selectedId ? 'bg-accent border-caramel/40' : 'border-transparent'"
          :aria-current="campaign.id === selectedId ? 'page' : undefined"
          @click="emit('navigate')"
        >
          <span class="flex items-center justify-between gap-2">
            <CampaignStatusBadge :status="campaign.status" />
            <time class="text-muted-foreground text-xs" :datetime="campaign.created_at">
              {{ d(campaign.created_at, 'short') }}
            </time>
          </span>
          <span
            class="line-clamp-2 break-words"
            :class="campaign.title ? 'font-medium' : ''"
            :title="campaign.title ? campaign.brief : undefined"
          >
            {{ campaign.title || campaign.brief }}
          </span>
        </RouterLink>
      </li>
      <li v-if="hasMore">
        <Button
          variant="outline"
          size="sm"
          class="w-full"
          :disabled="loadingMore"
          @click="emit('load-more')"
        >
          {{ loadingMore ? t('common.loading') : t('list.loadMore') }}
        </Button>
      </li>
    </ul>
  </section>
</template>
