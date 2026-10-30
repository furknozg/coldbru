<script setup lang="ts">
import { useMediaQuery } from '@vueuse/core'
import { MenuIcon } from '@lucide/vue'
import { computed, onMounted, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'

import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import CampaignSidebar from '@/features/campaigns/components/CampaignSidebar.vue'
import { provideCampaignList } from '@/features/campaigns/composables/campaignList'
import { useCampaignAnalytics } from '@/features/analytics/useCampaignAnalytics'

/**
 * Two columns (D-089): campaigns on the left, the routed page (the chat) beside it. Below 1024px
 * the list moves into a drawer opened from the header.
 */
const { t } = useI18n()
const route = useRoute()
const isDesktop = useMediaQuery('(min-width: 1024px)')

const list = provideCampaignList()

const selectedId = computed(() => (route.name === 'campaign' ? String(route.params.id) : null))
const selectedStatus = computed(
  () => list.items.value.find((c) => c.id === selectedId.value)?.status ?? null,
)

// The figures move when the campaign does (sending, completed), so the status triggers a refetch.
const analytics = useCampaignAnalytics(selectedId, selectedStatus)
const analyticsState = computed(() => ({
  report: analytics.report.value,
  loading: analytics.loading.value,
  failed: analytics.failed.value,
}))

const listOpen = shallowRef(false)

onMounted(() => void list.load())
</script>

<template>
  <div class="bg-background text-foreground flex h-dvh">
    <aside v-if="isDesktop" class="w-72 shrink-0 border-r" :aria-label="t('list.title')">
      <CampaignSidebar :selected-id="selectedId" :analytics="analyticsState" />
    </aside>

    <div class="flex min-w-0 flex-1 flex-col">
      <header v-if="!isDesktop" class="flex h-14 shrink-0 items-center gap-3 px-4">
        <Button
          variant="ghost"
          size="icon"
          :aria-label="t('list.open')"
          @click="listOpen = true"
        >
          <MenuIcon />
        </Button>
        <img src="/logo.png" :alt="t('app.logoAlt')" class="h-12 w-auto" />
      </header>
      <main class="min-h-0 flex-1">
        <RouterView />
      </main>
    </div>

    <Sheet v-if="!isDesktop" v-model:open="listOpen">
      <SheetContent side="left" class="p-0">
        <SheetHeader class="sr-only">
          <SheetTitle>{{ t('list.title') }}</SheetTitle>
          <SheetDescription>{{ t('list.drawerDescription') }}</SheetDescription>
        </SheetHeader>
        <CampaignSidebar
          :selected-id="selectedId"
          :analytics="analyticsState"
          @navigate="listOpen = false"
        />
      </SheetContent>
    </Sheet>
  </div>
</template>
