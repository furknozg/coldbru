<script setup lang="ts">
import { PlusIcon } from '@lucide/vue'
import { computed, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, type RouteLocationRaw } from 'vue-router'

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Button } from '@/components/ui/button'
import CampaignList from '@/features/campaigns/components/CampaignList.vue'
import { useCampaignList } from '@/features/campaigns/composables/campaignList'
import MenuCampaigns from '@/features/dashboard/components/MenuCampaigns.vue'
import { APP_MENU, type MenuLink } from '@/features/dashboard/menu'

/**
 * The one left column of the app (D-099, D-100), the same on the dashboard and in the chat: the
 * logo and a `+` for a new campaign, then the entries of `APP_MENU` in order, "Genel bakış", the
 * campaign list and the "Analitikler" group. The page that is open is highlighted and the groups
 * that lead to it are unfolded.
 */
const emit = defineEmits<{ navigate: [] }>()

const { t } = useI18n()
const route = useRoute()
const { items, loading, loadingMore, hasMore, error, load, loadMore } = useCampaignList()

const selectedId = computed(() => (route.name === 'campaign' ? String(route.params.id) : null))
const isCurrent = (link: MenuLink) => route.name === link.route

function to(link: MenuLink): RouteLocationRaw {
  const { from, to } = route.query
  return {
    name: link.route,
    query: link.keepsRange ? { ...(from ? { from } : {}), ...(to ? { to } : {}) } : {},
  }
}

// The groups that lead to the open page unfold; the reader can fold them and they stay folded
// until the page changes. Ids of groups and of the items inside them share one list.
const opened = shallowRef<string[]>([])
const isOpen = (id: string) => opened.value.includes(id)
function setOpen(id: string, open: boolean) {
  const rest = opened.value.filter((x) => x !== id)
  opened.value = open ? [...rest, id] : rest
}
watch(
  () => route.name,
  () => {
    for (const entry of APP_MENU) {
      if (entry.kind !== 'group') continue
      for (const child of entry.children.filter(isCurrent)) {
        setOpen(entry.id, true)
        if (child.campaigns) setOpen(child.id, true)
      }
    }
  },
  { immediate: true },
)
// The campaign of the open page, for the item that lists campaigns.
const selectedFor = (link: MenuLink) =>
  isCurrent(link) ? String(route.params.id ?? '') || null : null

// no-underline! (important): the accordion content underlines every link inside it.
const linkClass =
  'hover:bg-accent focus-visible:ring-ring/50 no-underline! flex items-center gap-2 rounded-lg py-1.5 text-sm font-medium outline-none focus-visible:ring-3'
</script>

<template>
  <div class="flex h-full min-h-0 flex-col gap-3 p-3">
    <div class="flex items-center justify-between gap-2">
      <RouterLink :to="{ name: 'dashboard' }" :aria-label="t('app.home')" @click="emit('navigate')">
        <img src="/logo.png" :alt="t('app.logoAlt')" class="h-12 w-auto" />
      </RouterLink>
      <Button as-child variant="outline" size="icon" :title="t('list.new')">
        <RouterLink
          :to="{ name: 'new-campaign' }"
          :aria-label="t('list.new')"
          @click="emit('navigate')"
        >
          <PlusIcon />
        </RouterLink>
      </Button>
    </div>

    <nav class="flex min-h-0 flex-1 flex-col gap-0.5" :aria-label="t('dashboard.menu.label')">
      <template v-for="entry in APP_MENU" :key="entry.id">
        <RouterLink
          v-if="entry.kind === 'link'"
          :to="to(entry)"
          :data-menu="entry.id"
          :class="[linkClass, isCurrent(entry) ? 'bg-accent' : '']"
          :aria-current="isCurrent(entry) ? 'page' : undefined"
          @click="emit('navigate')"
        >
          <component :is="entry.icon" class="text-caramel size-4" aria-hidden="true" />
          {{ t(entry.label) }}
        </RouterLink>
        <!-- flex: 0 1 auto: folded it is only as tall as its trigger, unfolded it grows to its content
             and, in a short sidebar, shrinks and scrolls inside, so the entries around it stay visible. -->
        <CampaignList
          v-else-if="entry.kind === 'campaigns'"
          :items="items"
          :selected-id="selectedId"
          :loading="loading"
          :loading-more="loadingMore"
          :has-more="hasMore"
          :error="error"
          :data-menu="entry.id"
          class="min-h-0 flex-initial overflow-y-auto"
          @navigate="emit('navigate')"
          @load-more="loadMore()"
          @retry="load()"
        />

        <Accordion
          v-else
          type="multiple"
          :model-value="isOpen(entry.id) ? [entry.id] : []"
          :data-menu="entry.id"
          @update:model-value="(value) => setOpen(entry.id, Boolean(value?.length))"
        >
          <AccordionItem :value="entry.id" class="border-b-0">
            <AccordionTrigger class="py-1.5">
              <span class="flex items-center gap-2">
                <component :is="entry.icon" class="text-caramel size-4" aria-hidden="true" />
                {{ t(entry.label) }}
              </span>
            </AccordionTrigger>
            <AccordionContent>
              <ul class="flex flex-col gap-0.5 pl-6">
                <li v-for="child in entry.children" :key="child.id" :data-menu="child.id">
                  <MenuCampaigns
                    v-if="child.campaigns"
                    :link="child as MenuLink & Required<Pick<MenuLink, 'campaigns'>>"
                    :items="items"
                    :loading="loading"
                    :error="error"
                    :selected-id="selectedFor(child)"
                    :open="isOpen(child.id)"
                    @update:open="(value) => setOpen(child.id, value)"
                    @navigate="emit('navigate')"
                    @retry="load()"
                  />
                  <RouterLink
                    v-else
                    :to="to(child)"
                    :class="[linkClass, 'px-2', isCurrent(child) ? 'bg-accent' : '']"
                    :aria-current="isCurrent(child) ? 'page' : undefined"
                    @click="emit('navigate')"
                  >
                    <component :is="child.icon" class="text-caramel size-4" aria-hidden="true" />
                    {{ t(child.label) }}
                  </RouterLink>
                </li>
              </ul>
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      </template>
    </nav>
  </div>
</template>
