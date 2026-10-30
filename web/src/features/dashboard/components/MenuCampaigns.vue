<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import CampaignRow from '@/features/campaigns/components/CampaignRow.vue'
import { toProblem } from '@/features/campaigns/problem'
import type { CampaignInfo } from '@/features/campaigns/types'

import type { MenuLink } from '../menu'
import DashboardCampaignGroups from './DashboardCampaignGroups.vue'

/**
 * A menu item that unfolds into campaigns (D-100): "Kampanya analitiği" lists the sending and
 * completed ones split by status, "Canlı gönderim" the sending and scheduled ones. A row opens the
 * section page of that campaign; the open one is highlighted and its group is unfolded. Loading,
 * failure and emptiness are plain lines.
 */
const props = defineProps<{
  link: MenuLink & Required<Pick<MenuLink, 'campaigns'>>
  items: readonly CampaignInfo[]
  loading: boolean
  error: unknown
  /** The campaign whose page is open, when the page is this item's. */
  selectedId: string | null
}>()
const open = defineModel<boolean>('open', { required: true })
const emit = defineEmits<{ navigate: []; retry: [] }>()

const { t } = useI18n()

const shown = computed(() =>
  props.items.filter((c) => props.link.campaigns.statuses.includes(c.status)),
)
const message = computed(() => (props.error ? toProblem(props.error, t).message : ''))
const route = computed(() => props.link.route)
</script>

<template>
  <Accordion
    type="multiple"
    :model-value="open ? [link.id] : []"
    :data-menu="link.id"
    @update:model-value="(value) => (open = Boolean(value?.length))"
  >
    <AccordionItem :value="link.id" class="border-b-0">
      <AccordionTrigger class="py-1.5 pl-2 text-sm">
        <span class="flex items-center gap-2">
          <component :is="link.icon" class="text-caramel size-4" aria-hidden="true" />
          {{ t(link.label) }}
        </span>
      </AccordionTrigger>
      <AccordionContent>
        <div class="max-h-72 overflow-y-auto pl-6">
          <p
            v-if="loading && items.length === 0"
            role="status"
            class="text-muted-foreground text-sm"
          >
            {{ t('common.loading') }}
          </p>
          <p v-else-if="error && items.length === 0" role="alert" class="text-destructive text-sm">
            {{ message }}
            <button type="button" class="underline underline-offset-2" @click="emit('retry')">
              {{ t('common.retry') }}
            </button>
          </p>
          <p v-else-if="shown.length === 0" class="text-muted-foreground text-sm">
            {{ t(link.campaigns.empty) }}
          </p>
          <DashboardCampaignGroups
            v-else-if="link.campaigns.grouped"
            :items="shown"
            :selected-id="selectedId"
            :route="route"
            :open="link.campaigns.statuses"
            :order="link.campaigns.statuses"
            @navigate="emit('navigate')"
          />
          <ul v-else class="flex flex-col gap-0.5 pl-1">
            <CampaignRow
              v-for="campaign in shown"
              :key="campaign.id"
              :campaign="campaign"
              :selected="campaign.id === selectedId"
              :to="{ name: route, params: { id: campaign.id } }"
              @navigate="emit('navigate')"
            />
          </ul>
        </div>
      </AccordionContent>
    </AccordionItem>
  </Accordion>
</template>
