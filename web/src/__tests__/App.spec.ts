import { flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { CampaignReport } from '@/api/analytics/service'
import * as analytics from '@/api/analytics/service'
import * as proposal from '@/api/proposal/service'
import { resetToasts } from '@/features/campaigns/composables/useToasts'

import { FakeEventSource, infoOf, mountApp, viewOf } from './helpers'

vi.mock('@/api/proposal/service')
vi.mock('@/api/analytics/service')

const report = (calls: number, cost: string): CampaignReport => ({
  campaign_id: 'c1',
  completed_at: '2026-10-30T10:00:00Z',
  deliveries: [],
  delivery_cost_usd: '0',
  llm: {
    calls,
    cost_usd: cost,
    by_purpose: [],
    tokens: { input: 0, output: 0, cache_read: 0, cache_creation: 0 },
  },
  sms_segments: 0,
})

let wrapper: Awaited<ReturnType<typeof mountApp>>['wrapper'] | undefined

beforeEach(() => {
  FakeEventSource.install()
  vi.mocked(proposal.getLlmMode).mockResolvedValue('mock')
  vi.mocked(proposal.listCampaigns).mockResolvedValue([])
  vi.mocked(analytics.getCampaignSummary).mockResolvedValue(null)
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  resetToasts()
  vi.resetAllMocks()
})

async function open(path = '/') {
  const mounted = await mountApp(path)
  wrapper = mounted.wrapper
  return mounted
}

describe('app shell', () => {
  it('draws the logo, a + for a new campaign, the title and the LLM mode, with no profile block, search or filter', async () => {
    const { wrapper } = await open()

    expect(wrapper.get('h1').text()).toBe('Smart Campaign Composer')
    expect(wrapper.get('img[src="/logo.png"]').attributes('alt')).toBe('coldbru')
    const plus = wrapper.get('aside a[aria-label="Yeni kampanya"]')
    expect(plus.attributes('href')).toBe('/')
    expect(plus.find('svg').exists()).toBe(true)
    expect(plus.text()).toBe('')
    expect(wrapper.find('input[type="search"]').exists()).toBe(false)
    expect(wrapper.find('aside select').exists()).toBe(false)
    expect(wrapper.get('[data-mode]').text()).toBe('LLM: mock')
    expect(wrapper.get('[data-mode]').classes()).toContain('bg-primary')
    expect(wrapper.text()).not.toMatch(/profil/i)
  })

  it('leaves the badge out when the service cannot be asked', async () => {
    vi.mocked(proposal.getLlmMode).mockResolvedValue(null)
    const { wrapper } = await open()

    expect(wrapper.find('[data-mode]').exists()).toBe(false)
  })

  it('shows the empty state with the brief box on /', async () => {
    const { wrapper } = await open('/')

    expect(wrapper.text()).toContain('Henüz kampanya yok.')
    expect(wrapper.find('textarea').exists()).toBe(true)
  })
})

describe('campaign list', () => {
  const campaigns = [
    infoOf({
      id: 'a',
      brief: 'İstanbul sepet hatırlatması',
      status: 'completed',
      created_at: '2026-10-30T09:00:00Z',
    }),
    infoOf({
      id: 'b',
      brief: 'Ankara yaz indirimi',
      status: 'drafting',
      created_at: '2026-10-29T09:00:00Z',
    }),
    infoOf({
      id: 'c',
      brief: 'İzmir hafta sonu kampanyası',
      status: 'cancelled',
      created_at: '2026-10-28T09:00:00Z',
    }),
  ]

  it('lists the campaigns as returned, newest first, with status, brief and time', async () => {
    vi.mocked(proposal.listCampaigns).mockResolvedValue(campaigns)
    const { wrapper } = await open()

    const rows = wrapper.findAll('aside li')
    expect(rows.map((r) => r.text())).toEqual([
      expect.stringContaining('İstanbul sepet hatırlatması'),
      expect.stringContaining('Ankara yaz indirimi'),
      expect.stringContaining('İzmir hafta sonu kampanyası'),
    ])
    expect(rows[0]!.text()).toContain('Tamamlandı')
    expect(rows[1]!.text()).toContain('Taslak')
    expect(rows[2]!.text()).toContain('İptal edildi')
    expect(rows[0]!.text()).toMatch(/30 Eki 2026/)
    expect(proposal.listCampaigns).toHaveBeenCalledWith(0)
  })

  it('shows the title of a campaign that has one, the brief otherwise', async () => {
    vi.mocked(proposal.listCampaigns).mockResolvedValue([
      infoOf({ id: 'a', brief: 'Uzun bir brief metni', title: 'Sepet hatırlatması' }),
      infoOf({ id: 'b', brief: 'Başlıksız brief' }),
    ])
    const { wrapper } = await open()

    const rows = wrapper.findAll('aside li')
    expect(rows[0]!.text()).toContain('Sepet hatırlatması')
    expect(rows[0]!.text()).not.toContain('Uzun bir brief metni')
    expect(rows[1]!.text()).toContain('Başlıksız brief')
  })

  it('loads the next page from the offset of what the server already handed out', async () => {
    const page = Array.from({ length: 20 }, (_, i) =>
      infoOf({ id: `p1-${i}`, brief: `Brief ${i}` }),
    )
    const second = [infoOf({ id: 'p2-0', brief: 'Brief son' })]
    vi.mocked(proposal.listCampaigns).mockResolvedValueOnce(page).mockResolvedValueOnce(second)
    const { wrapper } = await open()

    const more = wrapper.findAll('button').find((b) => b.text() === 'Daha fazla yükle')!
    await more.trigger('click')
    await flushPromises()

    expect(proposal.listCampaigns).toHaveBeenLastCalledWith(20)
    expect(wrapper.findAll('aside li')).toHaveLength(21)
    expect(wrapper.text()).toContain('Brief son')
    // A page shorter than the limit was the last one.
    expect(wrapper.text()).not.toContain('Daha fazla yükle')
  })

  it('shows why the list is empty when it fails and lets the user retry', async () => {
    vi.mocked(proposal.listCampaigns).mockRejectedValueOnce(new TypeError('offline'))
    const { wrapper } = await open()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)

    vi.mocked(proposal.listCampaigns).mockResolvedValue(campaigns)
    await wrapper
      .findAll('button')
      .find((b) => b.text() === 'Yeniden dene')!
      .trigger('click')
    await flushPromises()

    expect(wrapper.findAll('aside li')).toHaveLength(3)
  })
})

describe('analytics block', () => {
  it('shows the LLM call count and cost of the selected campaign', async () => {
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf({ id: 'c1' }))
    vi.mocked(analytics.getCampaignSummary).mockResolvedValue(report(12, '0.01234500'))
    const { wrapper } = await open('/campaigns/c1')

    const block = wrapper.get('section[aria-label="Analiz"]')
    expect(block.text()).toContain('LLM çağrısı')
    expect(block.text()).toContain('12')
    expect(block.text()).toContain('LLM maliyeti')
    expect(block.text()).toContain('$0,012345')
    expect(analytics.getCampaignSummary).toHaveBeenCalledWith('c1')
  })

  it('says there is no data yet when Analytics answers 404', async () => {
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf({ id: 'c1' }))
    const { wrapper } = await open('/campaigns/c1')

    expect(wrapper.get('section[aria-label="Analiz"]').text()).toContain('Henüz veri yok')
  })

  it('is absent while no campaign is selected', async () => {
    const { wrapper } = await open('/')

    expect(wrapper.find('section[aria-label="Analiz"]').exists()).toBe(false)
  })
})
