<script setup lang="ts">
import { PlusIcon } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import type { CampaignReport } from '@/api/analytics/service'
import { Button } from '@/components/ui/button'
import AnalyticsBlock from '@/features/analytics/components/AnalyticsBlock.vue'

import { useCampaignList } from '../composables/campaignList'
import CampaignList from './CampaignList.vue'

/** The left column: logo, the new-campaign button, the searchable list and the analytics block. */
defineProps<{
  selectedId: string | null
  analytics: { report: CampaignReport | null; loading: boolean; failed: boolean }
}>()

const emit = defineEmits<{ navigate: [] }>()

const { t } = useI18n()
const { visible, items, loading, loadingMore, hasMore, error, query, status, load, loadMore } =
  useCampaignList()
</script>

<template>
  <div class="flex h-full min-h-0 flex-col gap-3 p-3">
    <RouterLink to="/" class="self-start" :aria-label="t('app.home')" @click="emit('navigate')">
      <img src="/logo.png" :alt="t('app.logoAlt')" class="h-12 w-auto" />
    </RouterLink>
    <Button as-child>
      <RouterLink to="/" @click="emit('navigate')"><PlusIcon />{{ t('list.new') }}</RouterLink>
    </Button>
    <CampaignList
      v-model:query="query"
      v-model:status="status"
      :items="visible"
      :total="items.length"
      :selected-id="selectedId"
      :loading="loading"
      :loading-more="loadingMore"
      :has-more="hasMore"
      :error="error"
      @navigate="emit('navigate')"
      @load-more="loadMore()"
      @retry="load()"
    />
    <AnalyticsBlock
      v-if="selectedId"
      :report="analytics.report"
      :loading="analytics.loading"
      :failed="analytics.failed"
    />
  </div>
</template>
