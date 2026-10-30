import { computed, shallowRef } from 'vue'

import { CAMPAIGN_PAGE_SIZE, listCampaigns } from '@/api/proposal/service'

import type { CampaignInfo, CampaignStatus } from '../types'

export type StatusFilter = CampaignStatus | 'all'

const fold = (text: string) => text.toLocaleLowerCase('tr-TR')

/**
 * The campaign list of the left column: pages loaded newest first, searched and filtered in the
 * browser (D-087: the API has no search, so only the loaded pages are searched).
 */
export function useCampaigns() {
  const items = shallowRef<readonly CampaignInfo[]>([])
  const loading = shallowRef(false)
  const loadingMore = shallowRef(false)
  const error = shallowRef<unknown>(null)
  const hasMore = shallowRef(false)
  const query = shallowRef('')
  const status = shallowRef<StatusFilter>('all')

  /** How many rows the server has handed out; the next page starts here, not at `items.length`. */
  let fetched = 0
  let generation = 0

  const visible = computed(() => {
    const needle = fold(query.value.trim())
    return items.value.filter(
      (c) =>
        (status.value === 'all' || c.status === status.value) &&
        (needle === '' || fold(c.brief).includes(needle)),
    )
  })

  async function load() {
    const mine = ++generation
    loading.value = true
    error.value = null
    try {
      const page = await listCampaigns(0)
      if (mine !== generation) return
      items.value = page
      fetched = page.length
      hasMore.value = page.length === CAMPAIGN_PAGE_SIZE
    } catch (cause) {
      if (mine === generation) error.value = cause
    } finally {
      if (mine === generation) loading.value = false
    }
  }

  async function loadMore() {
    if (loading.value || loadingMore.value || !hasMore.value) return
    const mine = generation
    loadingMore.value = true
    try {
      const page = await listCampaigns(fetched)
      if (mine !== generation) return
      const known = new Set(items.value.map((c) => c.id))
      items.value = [...items.value, ...page.filter((c) => !known.has(c.id))]
      fetched += page.length
      hasMore.value = page.length === CAMPAIGN_PAGE_SIZE
    } catch (cause) {
      if (mine === generation) error.value = cause
    } finally {
      loadingMore.value = false
    }
  }

  /** Replaces the row of a campaign the page now knows better, or puts a new one on top. */
  function upsert(info: CampaignInfo) {
    const at = items.value.findIndex((c) => c.id === info.id)
    if (at === -1) {
      // Newest first: a campaign opened by its link lands where its age puts it, not on top.
      const before = items.value.findIndex((c) => c.created_at < info.created_at)
      const next = items.value.slice()
      next.splice(before === -1 ? next.length : before, 0, info)
      items.value = next
      return
    }
    const current = items.value[at]!
    if (
      current.status === info.status &&
      current.brief === info.brief &&
      current.updated_at === info.updated_at
    ) {
      return
    }
    const next = items.value.slice()
    next[at] = info
    items.value = next
  }

  return {
    items: computed(() => items.value),
    visible,
    loading: computed(() => loading.value),
    loadingMore: computed(() => loadingMore.value),
    error: computed(() => error.value),
    hasMore: computed(() => hasMore.value),
    query,
    status,
    load,
    loadMore,
    upsert,
  }
}

export type CampaignList = ReturnType<typeof useCampaigns>
