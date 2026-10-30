<script setup lang="ts">
import { useMediaQuery } from '@vueuse/core'
import { ChevronsLeftIcon, ListChecksIcon } from '@lucide/vue'
import { computed, shallowRef, watch } from 'vue'
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
import { Skeleton } from '@/components/ui/skeleton'
import BriefComposer from '@/features/campaigns/components/BriefComposer.vue'
import CampaignStatusBadge from '@/features/campaigns/components/CampaignStatusBadge.vue'
import CampaignTimeline from '@/features/campaigns/components/CampaignTimeline.vue'
import StepPanel from '@/features/campaigns/components/StepPanel.vue'
import { provideCampaignActions } from '@/features/campaigns/composables/campaignActions'
import { useCampaignList } from '@/features/campaigns/composables/campaignList'
import { useCampaign } from '@/features/campaigns/composables/useCampaign'
import { useLabels } from '@/features/campaigns/labels'
import { toProblem, type Problem } from '@/features/campaigns/problem'
import { buildTimeline } from '@/features/campaigns/timeline'
import { MAX_NOTE_CHARS } from '@/features/campaigns/types'

/**
 * `/campaigns/:id`: the timeline and note box in the centre, the step panel beside it (a drawer
 * below 1024px). The page owns the campaign state; the step panel and its cards reach the actions
 * through the provider set up here.
 */
const { t, te, d } = useI18n()
const labels = useLabels()
const route = useRoute()
const list = useCampaignList()
const isDesktop = useMediaQuery('(min-width: 1024px)')

const id = computed(() => String(route.params.id))
const campaign = useCampaign(id)
const view = campaign.view

const actions = provideCampaignActions(() => id.value, {
  applyView: campaign.applyView,
  refresh: campaign.refresh,
})

// The list shows the status of the campaign that is open, so a cancel or a sent campaign shows there.
watch(view, (current) => {
  if (!current) return
  list.upsert({
    id: current.id,
    brief: current.brief,
    status: current.status,
    created_at: current.created_at,
    updated_at: current.updated_at,
  })
})

const timeline = computed(() => buildTimeline(view.value, campaign.live.value, { t, te }, labels))

// Step panel state. `focusedStepId` is the step a timeline card or the step list picked.
const focusedStepId = shallowRef<string | null>(null)
const panelCollapsed = shallowRef(false)
const panelClosed = shallowRef(false)
const drawerOpen = shallowRef(false)

const openStepId = computed(() => view.value?.steps.find((s) => s.status === 'open')?.id ?? null)

// A step that opens is the thing to look at: follow it and bring the panel back if it was closed.
watch(openStepId, () => {
  focusedStepId.value = null
  panelClosed.value = false
})

watch(id, () => {
  focusedStepId.value = null
  panelClosed.value = false
  panelCollapsed.value = false
  drawerOpen.value = false
  note.value = ''
  noteProblem.value = null
})

function focusStep(stepId: string) {
  focusedStepId.value = stepId
  panelClosed.value = false
  panelCollapsed.value = false
  drawerOpen.value = true
}

const note = shallowRef('')
const noteProblem = shallowRef<Problem | null>(null)
const drafting = computed(() => view.value?.status === 'drafting')

async function addNote(text: string) {
  noteProblem.value = null
  const result = await actions.addContext(text)
  if (result.ok) note.value = ''
  else noteProblem.value = result.problem.status === 422 ? result.problem : null
}

const problem = computed(() => (campaign.error.value ? toProblem(campaign.error.value, t) : null))
</script>

<template>
  <div class="flex h-full min-h-0">
    <section class="flex min-w-0 flex-1 flex-col" :aria-label="t('timeline.title')">
      <header v-if="view" class="flex items-center gap-3 border-b px-4 py-2">
        <CampaignStatusBadge :status="view.status" />
        <p class="min-w-0 flex-1 truncate text-sm" :title="view.brief">{{ view.brief }}</p>
        <time class="text-muted-foreground hidden text-xs sm:block" :datetime="view.created_at">
          {{ d(view.created_at, 'short') }}
        </time>
        <Button
          v-if="!isDesktop"
          variant="outline"
          size="sm"
          :aria-label="t('panel.open')"
          @click="drawerOpen = true"
        >
          <ListChecksIcon />{{ t('panel.title') }}
          <span v-if="openStepId" class="bg-primary size-2 rounded-full" aria-hidden="true" />
        </Button>
        <Button
          v-else-if="panelClosed"
          variant="outline"
          size="sm"
          :aria-label="t('panel.open')"
          @click="panelClosed = false"
        >
          <ListChecksIcon />{{ t('panel.title') }}
        </Button>
      </header>

      <p
        v-if="campaign.connection.value === 'error' && view"
        role="status"
        class="text-muted-foreground bg-muted/60 border-b px-4 py-1 text-xs"
      >
        {{ t('timeline.reconnecting') }}
      </p>

      <div
        v-if="!view && campaign.loading.value"
        class="flex flex-col gap-3 p-4"
        role="status"
        :aria-label="t('common.loading')"
      >
        <Skeleton class="h-12 w-2/3" />
        <Skeleton class="h-12 w-1/2 self-end" />
        <Skeleton class="h-12 w-2/3" />
      </div>

      <div
        v-else-if="campaign.notFound.value"
        role="alert"
        class="flex flex-col items-start gap-3 p-6"
      >
        <p class="font-medium">{{ t('campaign.notFound') }}</p>
        <Button as-child variant="outline" size="sm">
          <RouterLink to="/">{{ t('campaign.backHome') }}</RouterLink>
        </Button>
      </div>

      <div v-else-if="problem && !view" role="alert" class="flex flex-col items-start gap-3 p-6">
        <p class="text-destructive font-medium">{{ problem.message }}</p>
        <Button variant="outline" size="sm" @click="campaign.refresh()">{{
          t('common.retry')
        }}</Button>
      </div>

      <template v-else-if="view">
        <CampaignTimeline
          :items="timeline"
          :streaming="campaign.streaming.value"
          @focus-step="focusStep"
        />
        <div class="border-t p-4">
          <BriefComposer
            v-model="note"
            :label="t('campaign.noteLabel')"
            :placeholder="t('campaign.notePlaceholder')"
            :submit-label="t('campaign.noteSubmit')"
            :max-length="MAX_NOTE_CHARS"
            :busy="actions.pending.value === 'context'"
            :disabled="!drafting"
            :disabled-reason="t('campaign.noteDisabled')"
            :problem="noteProblem"
            @submit="addNote"
          />
        </div>
      </template>
    </section>

    <template v-if="view">
      <aside
        v-if="isDesktop && !panelClosed"
        class="shrink-0 border-l"
        :class="panelCollapsed ? 'w-12' : 'w-[24rem] xl:w-[28rem]'"
        :aria-label="t('panel.title')"
      >
        <div v-if="panelCollapsed" class="flex flex-col items-center gap-2 py-3">
          <Button
            variant="ghost"
            size="icon-sm"
            :aria-label="t('panel.expand')"
            @click="panelCollapsed = false"
          >
            <ChevronsLeftIcon />
          </Button>
          <span
            v-if="openStepId"
            class="bg-primary size-2 rounded-full"
            :title="t('panel.decisionWaiting')"
          />
        </div>
        <StepPanel
          v-else
          :view="view"
          :focused-step-id="focusedStepId"
          @focus-step="focusedStepId = $event"
          @collapse="panelCollapsed = true"
          @close="panelClosed = true"
        />
      </aside>

      <Sheet v-else-if="!isDesktop" v-model:open="drawerOpen">
        <SheetContent side="right" :show-close-button="false" class="w-full p-0 sm:max-w-md">
          <SheetHeader class="sr-only">
            <SheetTitle>{{ t('panel.title') }}</SheetTitle>
            <SheetDescription>{{ t('panel.drawerDescription') }}</SheetDescription>
          </SheetHeader>
          <StepPanel
            :view="view"
            :focused-step-id="focusedStepId"
            @focus-step="focusedStepId = $event"
            @collapse="drawerOpen = false"
            @close="drawerOpen = false"
          />
        </SheetContent>
      </Sheet>
    </template>
  </div>
</template>
