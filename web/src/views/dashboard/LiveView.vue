<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'

import { useCampaignList } from '@/features/campaigns/composables/campaignList'
import DashboardPage from '@/features/dashboard/components/DashboardPage.vue'
import LiveDeliveries from '@/features/dashboard/components/LiveDeliveries.vue'
import { LIVE_CAMPAIGN_STATUSES } from '@/features/dashboard/live'

/**
 * `/live/:id?` (D-100): the sending campaigns are listed in the menu and the page follows the
 * delivery counts of the one picked live. Without a pick it says what to do, or that
 * nothing is being sent.
 */
const { t } = useI18n()
const route = useRoute()
const list = useCampaignList()

const id = computed(() => (route.params.id ? String(route.params.id) : null))
const campaign = computed(() => list.items.value.find((c) => c.id === id.value) ?? null)
const anyLive = computed(() =>
  list.items.value.some((c) => (LIVE_CAMPAIGN_STATUSES as readonly string[]).includes(c.status)),
)
</script>

<template>
  <DashboardPage :title="t('dashboard.menu.live')">
    <template v-if="id">
      <p v-if="campaign" class="text-sm font-medium break-words" data-testid="campaign-title">
        {{ campaign.title || campaign.brief }}
      </p>
      <!-- Keyed by campaign: switching campaigns closes one stream and opens the next. -->
      <LiveDeliveries :key="id" :campaign-id="id" />
    </template>
    <p v-else-if="anyLive" class="text-muted-foreground text-sm">{{ t('dashboard.live.pick') }}</p>
    <p v-else class="text-muted-foreground text-sm">{{ t('dashboard.live.none') }}</p>
  </DashboardPage>
</template>
