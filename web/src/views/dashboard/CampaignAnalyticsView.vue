<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'

import { Button } from '@/components/ui/button'
import { useCampaignAnalytics } from '@/features/analytics/useCampaignAnalytics'
import { useCampaignList } from '@/features/campaigns/composables/campaignList'
import CampaignClicks from '@/features/dashboard/components/CampaignClicks.vue'
import CampaignSummary, {
  type SummarySection,
} from '@/features/dashboard/components/CampaignSummary.vue'
import DashboardPage from '@/features/dashboard/components/DashboardPage.vue'
import MetricSkeleton from '@/features/dashboard/components/MetricSkeleton.vue'
import { ANALYTICS_STATUSES } from '@/features/dashboard/live'

/**
 * `/analytics/campaigns/:id?` (D-100): the campaign is picked in the menu (sending and completed ones); the page shows its Analytics report with a link to its chat. A campaign Analytics has not
 * heard of yet (no model call, nothing completed) has no report, which is said in a line and is no
 * error. Until the report is there the page is a skeleton of it (D-111); a refetch keeps the old
 * report in place.
 */
const { t } = useI18n()
const route = useRoute()
const list = useCampaignList()

const id = computed(() => (route.params.id ? String(route.params.id) : null))
const campaign = computed(() => list.items.value.find((c) => c.id === id.value) ?? null)
// Analytics knows a campaign once its sending has started (D-099). A campaign the list shows in
// another status has nothing to ask for; one the list does not hold is asked for anyway, once the
// list has loaded (it may still turn out to be a draft).
const started = computed(
  () =>
    !campaign.value || (ANALYTICS_STATUSES as readonly string[]).includes(campaign.value.status),
)
const waiting = computed(() => !campaign.value && !list.loaded.value)
// The figures move while the campaign goes through sending and completed, so its status refetches.
const analytics = useCampaignAnalytics(
  () => (started.value && !waiting.value ? id.value : null),
  () => campaign.value?.status,
)

/** Metric blocks (D-108); the success metrics come first in the summary (D-111). */
const sections = computed<readonly SummarySection[]>(() => [
  {
    key: 'clicks',
    title: t('dashboard.click.title'),
    component: CampaignClicks,
    position: 'start',
  },
])
</script>

<template>
  <DashboardPage :title="t('dashboard.menu.campaigns')">
    <template v-if="id" #actions>
      <Button as-child variant="outline" size="sm">
        <RouterLink :to="{ name: 'campaign', params: { id } }">{{
          t('dashboard.campaign.openChat')
        }}</RouterLink>
      </Button>
    </template>

    <p v-if="!id" class="text-muted-foreground text-sm">{{ t('dashboard.campaign.pick') }}</p>
    <template v-else>
      <p v-if="campaign" class="text-sm font-medium break-words" data-testid="campaign-title">
        {{ campaign.title || campaign.brief }}
      </p>
      <p v-if="!started" class="text-muted-foreground text-sm" data-testid="not-started">
        {{ t('dashboard.campaign.notStarted') }}
      </p>
      <p
        v-else-if="analytics.failed.value && !analytics.report.value"
        role="alert"
        class="text-destructive text-sm"
      >
        {{ t('analytics.failed') }}
      </p>
      <MetricSkeleton
        v-else-if="waiting || (analytics.loading.value && !analytics.report.value)"
        :tiles="5"
        :charts="2"
      />
      <p v-else-if="!analytics.report.value" class="text-muted-foreground text-sm">
        {{ t('dashboard.campaign.noReport') }}
      </p>
      <CampaignSummary v-else :report="analytics.report.value" :sections="sections" />
    </template>
  </DashboardPage>
</template>
