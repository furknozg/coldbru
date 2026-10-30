<script setup lang="ts">
import { shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { createCampaign } from '@/api/proposal/service'
import { useCampaignList } from '@/features/campaigns/composables/campaignList'
import { useToasts } from '@/features/campaigns/composables/useToasts'
import BriefComposer from '@/features/campaigns/components/BriefComposer.vue'
import { toProblem, type Problem } from '@/features/campaigns/problem'
import { MAX_BRIEF_CHARS } from '@/features/campaigns/types'

/** `/`: nothing selected yet, so the page offers the one thing to do, writing a brief. */
const { t } = useI18n()
const router = useRouter()
const list = useCampaignList()
const toasts = useToasts()

const brief = shallowRef('')
const busy = shallowRef(false)
const problem = shallowRef<Problem | null>(null)

async function submit(text: string) {
  busy.value = true
  problem.value = null
  try {
    const campaign = await createCampaign(text)
    list.upsert(campaign)
    brief.value = ''
    await router.push({ name: 'campaign', params: { id: campaign.id } })
  } catch (cause) {
    const failure = toProblem(cause, t)
    // The brief stays in the box either way; a 422 is shown beside it, anything else as a notice.
    if (failure.status === 422) problem.value = failure
    else toasts.push(failure.message)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="mx-auto flex h-full max-w-2xl flex-col justify-center gap-6 px-4 py-8">
    <div class="flex flex-col gap-2">
      <h2 class="text-espresso text-2xl font-semibold">{{ t('home.title') }}</h2>
      <p class="text-muted-foreground text-sm">{{ t('home.description') }}</p>
      <p class="bg-muted/60 rounded-lg border p-3 text-sm break-words">
        <span class="text-muted-foreground block text-xs">{{ t('home.exampleLabel') }}</span>
        {{ t('home.example') }}
      </p>
    </div>
    <BriefComposer
      v-model="brief"
      :label="t('home.composerLabel')"
      :placeholder="t('home.composerPlaceholder')"
      :submit-label="t('home.submit')"
      :max-length="MAX_BRIEF_CHARS"
      :busy="busy"
      :problem="problem"
      @submit="submit"
    />
  </div>
</template>
