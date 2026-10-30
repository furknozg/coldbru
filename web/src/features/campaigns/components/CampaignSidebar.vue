<script setup lang="ts">
import { PlusIcon } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import type { CampaignReport } from '@/api/analytics/service'
import { Button } from '@/components/ui/button'
import AnalyticsBlock from '@/features/analytics/components/AnalyticsBlock.vue'

import { useCampaignList } from '../composables/campaignList'
import CampaignList from './CampaignList.vue'

/** The left column (D-089): logo, a `+` for a new campaign, the campaign list, the analytics block. */
defineProps<{
  selectedId: string | null
  analytics: { report: CampaignReport | null; loading: boolean; failed: boolean }
}>()

const emit = defineEmits<{ navigate: [] }>()

const { t } = useI18n()
const { items, loading, loadingMore, hasMore, error, load, loadMore } = useCampaignList()
</script>

<template>
  <div class="flex h-full min-h-0 flex-col gap-3 p-3">
    <div class="flex items-center justify-between gap-2">
      <RouterLink to="/" :aria-label="t('app.home')" @click="emit('navigate')">
        <img src="/logo.png" :alt="t('app.logoAlt')" class="h-12 w-auto" />
      </RouterLink>
      <Button as-child variant="outline" size="icon" :title="t('list.new')">
        <RouterLink to="/" :aria-label="t('list.new')" @click="emit('navigate')">
          <PlusIcon />
        </RouterLink>
      </Button>
    </div>
    <CampaignList
      :items="items"
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
