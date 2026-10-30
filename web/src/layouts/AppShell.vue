<script setup lang="ts">
import { useMediaQuery } from '@vueuse/core'
import { MenuIcon } from '@lucide/vue'
import { onMounted, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { provideCampaignList } from '@/features/campaigns/composables/campaignList'
import AppSidebar from './AppSidebar.vue'

/**
 * Two columns (D-089, D-099): the one menu on the left, the routed page beside it. The menu is the
 * same on every route; it shares the campaign list that is loaded once here. Below 1024px the
 * menu moves into a drawer opened from the top bar, whose logo leads to the dashboard (D-094).
 */
const { t } = useI18n()
const isDesktop = useMediaQuery('(min-width: 1024px)')

const list = provideCampaignList()

const menuOpen = shallowRef(false)

onMounted(() => void list.load())
</script>

<template>
  <div class="bg-background text-foreground flex h-dvh">
    <aside v-if="isDesktop" class="w-72 shrink-0 border-r" :aria-label="t('dashboard.menu.label')">
      <AppSidebar />
    </aside>

    <div class="flex min-w-0 flex-1 flex-col">
      <header v-if="!isDesktop" class="flex h-14 shrink-0 items-center gap-3 px-4">
        <Button
          variant="ghost"
          size="icon"
          :aria-label="t('dashboard.menu.open')"
          @click="menuOpen = true"
        >
          <MenuIcon />
        </Button>
        <RouterLink :to="{ name: 'dashboard' }" :aria-label="t('app.home')">
          <img src="/logo.png" :alt="t('app.logoAlt')" class="h-12 w-auto" />
        </RouterLink>
      </header>
      <main class="min-h-0 flex-1">
        <RouterView />
      </main>
    </div>

    <Sheet v-if="!isDesktop" v-model:open="menuOpen">
      <SheetContent side="left" class="p-0">
        <SheetHeader class="sr-only">
          <SheetTitle>{{ t('dashboard.menu.label') }}</SheetTitle>
          <SheetDescription>{{ t('dashboard.menu.drawerDescription') }}</SheetDescription>
        </SheetHeader>
        <AppSidebar @navigate="menuOpen = false" />
      </SheetContent>
    </Sheet>
  </div>
</template>
