<script setup lang="ts">
import { Megaphone } from '@lucide/vue'
import { computed, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { groupByStatus } from '../groups'
import { toProblem } from '../problem'
import type { CampaignInfo, CampaignStatus } from '../types'
import CampaignRow from './CampaignRow.vue'

/**
 * The loaded campaigns (D-091): one accordion, "Kampanyalar", with a sub-accordion per status. A
 * status without a campaign has no group; the group of the open campaign and the drafts start open.
 */
const props = defineProps<{
  items: readonly CampaignInfo[]
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

const { t, n } = useI18n()

const ROOT = 'campaigns'
const rootOpen = shallowRef<string[]>([ROOT])
const groupsOpen = shallowRef<CampaignStatus[]>(['drafting'])

const groups = computed(() => groupByStatus(props.items))
const selectedStatus = computed(
  () => props.items.find((c) => c.id === props.selectedId)?.status ?? null,
)
const errorMessage = computed(() => (props.error ? toProblem(props.error, t).message : ''))

// Opening a campaign (or a status change of the open one) unfolds its group; a group the reader
// folded stays folded until then.
watch(
  selectedStatus,
  (status) => {
    if (status && !groupsOpen.value.includes(status)) {
      groupsOpen.value = [...groupsOpen.value, status]
    }
  },
  { immediate: true },
)
</script>

<template>
  <section :aria-label="t('list.title')">
    <Accordion v-model="rootOpen" type="multiple">
      <AccordionItem :value="ROOT" class="border-b-0">
        <AccordionTrigger class="py-1.5">
          <span class="flex items-center gap-2">
            <Megaphone class="text-caramel size-4" aria-hidden="true" />
            {{ t('list.title') }}
          </span>
        </AccordionTrigger>
        <AccordionContent>
          <div
            v-if="loading"
            class="flex flex-col gap-2"
            role="status"
            :aria-label="t('common.loading')"
          >
            <Skeleton v-for="k in 4" :key="k" class="h-8 w-full" />
          </div>

          <div
            v-else-if="error && items.length === 0"
            role="alert"
            class="text-destructive space-y-2 text-sm"
          >
            <p>{{ errorMessage }}</p>
            <Button variant="outline" size="sm" @click="emit('retry')">{{
              t('common.retry')
            }}</Button>
          </div>

          <p v-else-if="items.length === 0" class="text-muted-foreground text-sm">
            {{ t('list.empty') }}
          </p>

          <template v-else>
            <Accordion v-model="groupsOpen" type="multiple" class="pl-8">
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
                      @navigate="emit('navigate')"
                    />
                  </ul>
                </AccordionContent>
              </AccordionItem>
            </Accordion>
            <Button
              v-if="hasMore"
              variant="outline"
              size="sm"
              class="mt-2 w-full"
              :disabled="loadingMore"
              @click="emit('load-more')"
            >
              {{ loadingMore ? t('common.loading') : t('list.loadMore') }}
            </Button>
          </template>
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  </section>
</template>
