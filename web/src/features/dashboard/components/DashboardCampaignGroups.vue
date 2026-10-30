<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import CampaignRow from '@/features/campaigns/components/CampaignRow.vue'
import { groupByStatus } from '@/features/campaigns/groups'
import type { CampaignInfo, CampaignStatus } from '@/features/campaigns/types'

/**
 * Campaigns split by status like the list of the chat screen (D-091); a status without a campaign
 * has no group. The group of the selected campaign is open, and so are the
 * groups in `open` (the section decides which are worth showing at once).
 */
const props = defineProps<{
  items: readonly CampaignInfo[]
  selectedId: string | null
  /** Route name a row links to; the campaign id is its `id` param. */
  route: string
  open?: readonly CampaignStatus[]
  /** The order of the groups; the order of the statuses everywhere else when left out. */
  order?: readonly CampaignStatus[]
}>()
const emit = defineEmits<{ navigate: [] }>()

const { t, n } = useI18n()

const groups = computed(() => {
  const all = groupByStatus(props.items)
  const order = props.order
  return order ? [...all].sort((a, b) => order.indexOf(a.status) - order.indexOf(b.status)) : all
})
const selectedStatus = computed(
  () => props.items.find((c) => c.id === props.selectedId)?.status ?? null,
)
const openGroups = shallowRef<CampaignStatus[]>([...(props.open ?? [])])

watch(
  selectedStatus,
  (status) => {
    if (status && !openGroups.value.includes(status))
      openGroups.value = [...openGroups.value, status]
  },
  { immediate: true },
)
</script>

<template>
  <Accordion v-model="openGroups" type="multiple">
    <AccordionItem
      v-for="group in groups"
      :key="group.status"
      :value="group.status"
      :data-group="group.status"
      class="border-b-0"
    >
      <AccordionTrigger class="py-1 text-xs">
        <span class="flex items-baseline gap-2">
          {{ t(`list.group.${group.status}`) }}
          <span class="text-muted-foreground font-normal tabular-nums">{{
            n(group.items.length, 'int')
          }}</span>
        </span>
      </AccordionTrigger>
      <AccordionContent>
        <ul class="flex flex-col gap-0.5 pl-1">
          <CampaignRow
            v-for="campaign in group.items"
            :key="campaign.id"
            :campaign="campaign"
            :selected="campaign.id === selectedId"
            :to="{ name: route, params: { id: campaign.id } }"
            @navigate="emit('navigate')"
          />
        </ul>
      </AccordionContent>
    </AccordionItem>
  </Accordion>
</template>
