<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import FormSelect from '@/components/FormSelect.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { toProblem } from '../problem'
import { useLabels } from '../labels'
import type { StatusFilter } from '../composables/useCampaigns'
import { CAMPAIGN_STATUSES, type CampaignInfo } from '../types'
import CampaignStatusBadge from './CampaignStatusBadge.vue'

const props = defineProps<{
  /** Rows after the search and the status filter. */
  items: readonly CampaignInfo[]
  /** Rows loaded, before filtering; tells an empty list from an empty search. */
  total: number
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

const query = defineModel<string>('query', { required: true })
const status = defineModel<StatusFilter>('status', { required: true })

const { t, d } = useI18n()
const labels = useLabels()

const statusOptions = computed(() => [
  { value: 'all' as const, label: t('list.allStatuses') },
  ...CAMPAIGN_STATUSES.map((value) => ({ value, label: labels.status(value) })),
])

const errorMessage = computed(() => (props.error ? toProblem(props.error, t).message : ''))
</script>

<template>
  <section class="flex min-h-0 flex-1 flex-col gap-2" :aria-label="t('list.title')">
    <div class="flex flex-col gap-2">
      <Input
        v-model="query"
        type="search"
        :placeholder="t('list.searchPlaceholder')"
        :aria-label="t('list.searchLabel')"
        maxlength="200"
      />
      <FormSelect v-model="status" :options="statusOptions" :aria-label="t('list.statusLabel')" />
    </div>

    <div v-if="loading" class="flex flex-col gap-2" role="status" :aria-label="t('common.loading')">
      <Skeleton v-for="n in 4" :key="n" class="h-16 w-full" />
    </div>

    <div v-else-if="error && total === 0" role="alert" class="text-destructive space-y-2 text-sm">
      <p>{{ errorMessage }}</p>
      <Button variant="outline" size="sm" @click="emit('retry')">{{ t('common.retry') }}</Button>
    </div>

    <p v-else-if="total === 0" class="text-muted-foreground text-sm">{{ t('list.empty') }}</p>

    <p v-else-if="items.length === 0" class="text-muted-foreground text-sm">
      {{ t('list.noMatch') }}
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
          <span class="line-clamp-2 break-words">{{ campaign.brief }}</span>
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
