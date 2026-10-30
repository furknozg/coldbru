<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'

import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import BriefComposer from '@/features/campaigns/components/BriefComposer.vue'
import CampaignStatusBadge from '@/features/campaigns/components/CampaignStatusBadge.vue'
import ChatStream from '@/features/campaigns/components/ChatStream.vue'
import { provideCampaignActions } from '@/features/campaigns/composables/campaignActions'
import { useCampaignList } from '@/features/campaigns/composables/campaignList'
import { useCampaign } from '@/features/campaigns/composables/useCampaign'
import { useLabels } from '@/features/campaigns/labels'
import { toProblem, type Problem } from '@/features/campaigns/problem'
import { buildTimeline } from '@/features/campaigns/timeline'
import { MAX_ANSWER_CHARS, MAX_NOTE_CHARS } from '@/features/campaigns/types'

/**
 * `/campaigns/:id`: one chat column (D-089). The title on top, the cards, the steps and their
 * actions in the stream, and the composer at the bottom, which sends a context note or, while the
 * agent waits for one, the answer to its question (D-090). The page owns the campaign state; the
 * cards reach the actions through the provider set up here.
 */
const { t, te, d } = useI18n()
const labels = useLabels()
const route = useRoute()
const list = useCampaignList()

const id = computed(() => String(route.params.id))
const campaign = useCampaign(id)
const view = campaign.view

const actions = provideCampaignActions(() => id.value, {
  applyView: campaign.applyView,
  refresh: campaign.refresh,
})

// The list shows the status and the title of the campaign that is open as they change.
watch(view, (current) => {
  if (!current) return
  list.upsert({
    id: current.id,
    brief: current.brief,
    title: current.title,
    status: current.status,
    created_at: current.created_at,
    updated_at: current.updated_at,
  })
})

const timeline = computed(() => buildTimeline(view.value, campaign.live.value, { t, te }, labels))

const drafting = computed(() => view.value?.status === 'drafting')
const pendingQuestion = computed(() => {
  if (!drafting.value) return null
  return view.value?.steps.find((s) => s.status === 'open')?.pending_question ?? null
})

const text = shallowRef('')
const textProblem = shallowRef<Problem | null>(null)

watch(id, () => {
  text.value = ''
  textProblem.value = null
})
// A question that is answered or withdrawn takes its half-typed answer away too.
watch(
  () => pendingQuestion.value?.id ?? null,
  () => {
    textProblem.value = null
  },
)

async function send(body: string) {
  textProblem.value = null
  const question = pendingQuestion.value
  const result = question ? await actions.answer(question.id, body) : await actions.addContext(body)
  if (result.ok) text.value = ''
  else textProblem.value = result.problem.status === 422 ? result.problem : null
}

const composerBusy = computed(() => {
  const pending = actions.pending.value
  return pending === 'context' || (pending?.startsWith('answer:') ?? false)
})

const problem = computed(() => (campaign.error.value ? toProblem(campaign.error.value, t) : null))
</script>

<template>
  <section class="flex h-full min-h-0 flex-col" :aria-label="t('timeline.title')">
    <header v-if="view" class="flex items-center gap-3 border-b px-4 py-2">
      <CampaignStatusBadge :status="view.status" />
      <h2
        class="text-espresso min-w-0 flex-1 truncate text-sm font-semibold"
        :title="view.title || view.brief"
        data-campaign-title
      >
        {{ view.title || view.brief }}
      </h2>
      <time class="text-muted-foreground hidden text-xs sm:block" :datetime="view.created_at">
        {{ d(view.created_at, 'short') }}
      </time>
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
      <ChatStream :view="view" :items="timeline" :streaming="campaign.streaming.value" />
      <div class="border-t p-4">
        <div class="mx-auto max-w-4xl">
          <BriefComposer
            v-if="pendingQuestion"
            v-model="text"
            :label="t('question.answerLabel')"
            :placeholder="t('question.answerPlaceholder')"
            :submit-label="t('question.answerSubmit')"
            :max-length="MAX_ANSWER_CHARS"
            :busy="composerBusy"
            :problem="textProblem"
            @submit="send"
          />
          <BriefComposer
            v-else
            v-model="text"
            :label="t('campaign.noteLabel')"
            :placeholder="t('campaign.notePlaceholder')"
            :submit-label="t('campaign.noteSubmit')"
            :max-length="MAX_NOTE_CHARS"
            :busy="composerBusy"
            :disabled="!drafting"
            :disabled-reason="t('campaign.noteDisabled')"
            :problem="textProblem"
            @submit="send"
          />
        </div>
      </div>
    </template>
  </section>
</template>
