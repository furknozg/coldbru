import { shallowRef, toValue, watch, type MaybeRefOrGetter } from 'vue'

import { getCampaignSummary, type CampaignReport } from '@/api/analytics/service'

/**
 * The LLM figures of a campaign (call count and cost, D-083). `report` stays `null` while Analytics
 * has nothing (a 404 is no error). `refreshOn` is any value whose change means the figures moved,
 * for example the campaign status.
 */
export function useCampaignAnalytics(
  campaignId: MaybeRefOrGetter<string | null>,
  refreshOn?: MaybeRefOrGetter<unknown>,
) {
  const report = shallowRef<CampaignReport | null>(null)
  const loading = shallowRef(false)
  const failed = shallowRef(false)
  let generation = 0

  watch(
    [() => toValue(campaignId), () => toValue(refreshOn)],
    async ([target], old) => {
      const mine = ++generation
      if (target !== old?.[0]) report.value = null
      failed.value = false
      if (!target) {
        loading.value = false
        return
      }
      loading.value = true
      try {
        const next = await getCampaignSummary(target)
        if (mine === generation) report.value = next ?? null
      } catch {
        if (mine === generation) failed.value = true
      } finally {
        if (mine === generation) loading.value = false
      }
    },
    { immediate: true },
  )

  return { report, loading, failed }
}
