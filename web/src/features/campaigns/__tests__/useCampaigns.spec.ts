import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import * as proposal from '@/api/proposal/service'
import { infoOf } from '@/__tests__/helpers'

import { useCampaigns } from '../composables/useCampaigns'

vi.mock('@/api/proposal/service')

beforeEach(() => {
  vi.mocked(proposal.listCampaigns).mockResolvedValue([])
})

afterEach(() => {
  vi.resetAllMocks()
})

describe('useCampaigns', () => {
  it('puts a campaign it did not know where its age belongs and replaces one it knew', async () => {
    const newest = infoOf({ id: 'new', created_at: '2026-10-30T10:00:00Z' })
    const oldest = infoOf({ id: 'old', created_at: '2026-10-01T10:00:00Z' })
    vi.mocked(proposal.listCampaigns).mockResolvedValue([newest, oldest])
    const list = useCampaigns()
    await list.load()

    list.upsert(infoOf({ id: 'mid', created_at: '2026-10-15T10:00:00Z' }))
    list.upsert(infoOf({ id: 'top', created_at: '2026-10-31T10:00:00Z' }))
    expect(list.items.value.map((c) => c.id)).toEqual(['top', 'new', 'mid', 'old'])

    list.upsert({ ...newest, status: 'cancelled', updated_at: '2026-10-30T11:00:00Z' })
    expect(list.items.value.find((c) => c.id === 'new')!.status).toBe('cancelled')
    expect(list.items.value).toHaveLength(4)
  })

  it('does not list a campaign twice when a new one shifted the page boundary', async () => {
    const page = Array.from({ length: 20 }, (_, i) => infoOf({ id: `a${i}` }))
    vi.mocked(proposal.listCampaigns)
      .mockResolvedValueOnce(page)
      .mockResolvedValueOnce([page[19]!, infoOf({ id: 'b0' })])
    const list = useCampaigns()
    await list.load()
    list.upsert(infoOf({ id: 'fresh', created_at: '2027-01-01T00:00:00Z' }))

    await list.loadMore()

    expect(vi.mocked(proposal.listCampaigns)).toHaveBeenLastCalledWith(20)
    const ids = list.items.value.map((c) => c.id)
    expect(new Set(ids).size).toBe(ids.length)
    expect(ids).toContain('b0')
  })
})
