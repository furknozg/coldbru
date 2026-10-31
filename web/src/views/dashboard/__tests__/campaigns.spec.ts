import { defineComponent, h } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import * as analytics from '@/api/analytics/service'
import * as proposal from '@/api/proposal/service'
import { FakeEventSource, infoOf, mountApp, viewOf } from '@/__tests__/helpers'
import { createAppI18n } from '@/i18n'
import { resetToasts } from '@/features/campaigns/composables/useToasts'
import CampaignSummary from '@/features/dashboard/components/CampaignSummary.vue'
import { reportOf } from '@/features/dashboard/__tests__/fixtures'

import { cardTitles, legend, mockDashboardApi, settle } from './harness'

vi.mock('@/api/proposal/service')
vi.mock('@/api/analytics/service')
vi.setConfig({ testTimeout: 20000 })

let wrapper: Awaited<ReturnType<typeof mountApp>>['wrapper'] | undefined

beforeEach(() => {
  FakeEventSource.install()
  mockDashboardApi()
  vi.mocked(proposal.listCampaigns).mockResolvedValue([
    infoOf({
      id: 'c1',
      brief: 'Sepet hatırlatması',
      title: 'Sepet kampanyası',
      status: 'completed',
    }),
    infoOf({ id: 'c2', brief: 'Henüz bitmeyen', status: 'sending' }),
    infoOf({ id: 'd1', brief: 'Taslak brief', status: 'drafting' }),
    infoOf({ id: 's1', brief: 'Zamanlanan brief', status: 'scheduled' }),
    infoOf({ id: 'x1', brief: 'İptal brief', status: 'cancelled' }),
  ])
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  resetToasts()
  vi.resetAllMocks()
})

async function open(path = '/analytics/campaigns/c1') {
  const mounted = await mountApp(path)
  await settle()
  wrapper = mounted.wrapper
  return mounted
}

describe('Kampanya analitiği', () => {
  it('asks for a pick while no campaign is selected', async () => {
    const { wrapper } = await open('/analytics/campaigns')
    expect(wrapper.get('main').text()).toContain('Menüden bir kampanya seçin.')
    expect(analytics.getCampaignSummary).not.toHaveBeenCalled()
    expect(wrapper.find('main a[href^="/campaigns/"]').exists()).toBe(false)
  })

  it('keeps the campaigns out of the content area: the menu holds them', async () => {
    const { wrapper } = await open('/analytics/campaigns')

    expect(wrapper.get('main').text()).toContain('Menüden bir kampanya seçin.')
    expect(wrapper.find('main [data-group]').exists()).toBe(false)
    expect(wrapper.find('main [data-testid="campaign-picker"]').exists()).toBe(false)
    // The page opens its own item in the menu, split into Gönderiliyor and Tamamlandı.
    const item = wrapper.get('aside [data-menu="campaign-analytics"]')
    expect(item.findAll('[data-group]').map((g) => g.attributes('data-group'))).toEqual([
      'sending',
      'completed',
    ])
  })

  it('lists only sending and completed campaigns: no draft, scheduled or cancelled one', async () => {
    const { wrapper } = await open('/analytics/campaigns')
    const item = wrapper.get('aside [data-menu="campaign-analytics"]')

    expect(item.findAll('a').map((a) => a.attributes('href'))).toEqual([
      '/analytics/campaigns/c2',
      '/analytics/campaigns/c1',
    ])
    for (const text of ['Taslak brief', 'Zamanlanan brief', 'İptal brief']) {
      expect(item.text()).not.toContain(text)
    }
  })

  it('opens the report of a campaign picked in the menu', async () => {
    const { wrapper, router } = await open('/analytics/campaigns')
    await wrapper
      .get('aside [data-menu="campaign-analytics"] [data-group="completed"] a')
      .trigger('click')
    await settle()

    expect(router.currentRoute.value.fullPath).toBe('/analytics/campaigns/c1')
    expect(analytics.getCampaignSummary).toHaveBeenCalledWith('c1')
    expect(wrapper.get('[data-testid="campaign-title"]').text()).toBe('Sepet kampanyası')
    expect(wrapper.get('aside nav a[aria-current="page"]').text()).toContain('Sepet kampanyası')
  })

  it.each(['drafting', 'scheduled', 'cancelled'] as const)(
    'says analytics comes once sending starts for a %s campaign opened by its link, and keeps the chat link',
    async (status) => {
      vi.mocked(proposal.listCampaigns).mockResolvedValue([
        infoOf({ id: 'z', brief: 'Brief', status }),
      ])
      const { wrapper } = await open('/analytics/campaigns/z')
      const main = wrapper.get('main')

      expect(main.get('[data-testid="not-started"]').text()).toBe(
        'Analitik, kampanyanın gönderimi başladıktan sonra görünür.',
      )
      expect(main.get('a[href="/campaigns/z"]').text()).toBe('Sohbeti aç')
      expect(analytics.getCampaignSummary).not.toHaveBeenCalled()
      expect(main.find('[data-stat]').exists()).toBe(false)
    },
  )

  it('asks Analytics for a campaign that is sending', async () => {
    await open('/analytics/campaigns/c2')
    expect(analytics.getCampaignSummary).toHaveBeenCalledWith('c2')
  })

  it('shows the completion time, the costs, the deliveries and the model spend of a completed campaign', async () => {
    const { wrapper } = await open()
    const main = wrapper.get('main')

    expect(main.get('[data-testid="completed-at"]').text()).toContain('30 Eki 2026')
    expect(main.get('[data-testid="completed-at"]').text()).not.toContain('Henüz tamamlanmadı')
    expect(main.get('[data-stat="deliveryCost"]').text()).toContain('$12,50')
    expect(main.get('[data-stat="segments"]').text()).toContain('75')
    expect(main.get('[data-stat="llmCalls"]').text()).toContain('6')
    expect(main.get('[data-stat="llmCost"]').text()).toContain('$0,0042')
    expect(main.get('[data-stat="tokens"]').text()).toContain('6.000')

    expect(legend(main, 'Kanala göre teslimat')).toEqual([
      ['E-posta', '100', '%66,7'],
      ['SMS', '50', '%33,3'],
    ])
    expect(legend(main, 'Duruma göre teslimat').map((row) => row.slice(0, 2))).toEqual([
      ['Gönderildi', '140'],
      ['Başarısız', '10'],
    ])
    expect(legend(main, 'Amaca göre maliyet').map((row) => row.slice(0, 2))).toEqual([
      ['Ön eleme', '$0,0002'],
      ['Metin', '$0,004'],
    ])
    expect(
      main.findAll('[data-row="purpose"]').map((row) => row.findAll('th, td').map((c) => c.text())),
    ).toEqual([
      ['Ön eleme', '1', '110', '$0,0002'],
      ['Metin', '5', '5.890', '$0,004'],
    ])
  })

  it('says "henüz tamamlanmadı" and draws no delivery pies for a campaign that has not completed', async () => {
    vi.mocked(analytics.getCampaignSummary).mockResolvedValue(
      reportOf({ campaign_id: 'c2', completed_at: null as unknown as string, deliveries: [] }),
    )
    const { wrapper } = await open('/analytics/campaigns/c2')
    const main = wrapper.get('main')

    expect(main.get('[data-testid="completed-at"]').text()).toBe('Henüz tamamlanmadı')
    const titles = main.findAll('figure').map((f) => f.find('figcaption').text())
    expect(titles).toEqual(['Amaca göre maliyet'])
  })

  it('shows the success metrics of a completed campaign: CTR, clicks, CTR per channel and the language and time zone table', async () => {
    const { wrapper } = await open()
    const section = wrapper.get('main [data-section="clicks"]')

    expect(section.get('h3').text()).toBe('Başarı metrikleri')
    expect(section.find('[data-testid="ctr-pending"]').exists()).toBe(false)
    expect(section.get('[data-stat="ctr"] dd').text()).toBe('%14,3')
    expect(section.get('[data-stat="uniqueClicks"] dd').text()).toBe('20')
    expect(section.get('[data-stat="totalClicks"] dd').text()).toBe('31')
    expect(section.get('[data-stat="ctr-email"] dd').text()).toBe('%16,7')
    expect(section.get('[data-stat="ctr-email"]').text()).toContain('tekil 15 / gönderilen 90')
    expect(section.get('[data-stat="ctr-sms"] dd').text()).toBe('%10')
    expect(
      section
        .findAll('[data-row="locale"]')
        .map((row) => row.findAll('th, td').map((c) => c.text())),
    ).toEqual([
      ['E-posta', 'Türkçe', 'Europe/Istanbul', '60', '12', '20', '%20'],
      ['E-posta', 'İngilizce', 'Europe/London', '30', '3', '4', '%10'],
      ['SMS', 'Türkçe', 'Europe/Istanbul', '50', '5', '7', '%10'],
    ])
  })

  it('renders the success metrics first, before the delivery figures and the LLM block, each group a card (D-111)', async () => {
    const { wrapper } = await open()

    const clicks = wrapper.get('main [data-section="clicks"]')
    const delivery = wrapper.get('main [data-stat="deliveryCost"]')
    const llm = wrapper.get('main [data-row="purpose"]')
    const before = (a: Element, b: Element) =>
      Boolean(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING)
    expect(before(clicks.element, delivery.element)).toBe(true)
    expect(before(delivery.element, llm.element)).toBe(true)
    // Each group is one card, and no card holds another.
    expect(clicks.attributes('data-slot')).toBe('metric-card')
    expect(delivery.element.closest('[data-slot="metric-card"]')).not.toBeNull()
    expect(llm.element.closest('[data-slot="metric-card"]')).not.toBeNull()
    for (const card of wrapper.findAll('main [data-slot="metric-card"]')) {
      expect(card.find('[data-slot="metric-card"]').exists()).toBe(false)
    }
  })

  it('titles every card of the report', async () => {
    const { wrapper } = await open()
    const main = wrapper.get('main')

    expect(cardTitles(main)).toEqual([
      'Başarı metrikleri',
      'Maliyet ve kullanım',
      'Teslimat dağılımı',
      'Amaca göre LLM kullanımı',
    ])
    expect(main.findAll('h3').map((h) => h.text())).toEqual(cardTitles(main))
  })

  it('shows a skeleton of the report, not a line of text, while it loads (D-111)', async () => {
    let resolve!: (report: ReturnType<typeof reportOf>) => void
    vi.mocked(analytics.getCampaignSummary).mockReturnValue(
      new Promise((r) => {
        resolve = r
      }),
    )
    const { wrapper } = await open()

    const status = wrapper.get('main [role="status"]')
    expect(status.findAll('[data-slot="skeleton"]').length).toBeGreaterThan(0)
    expect(status.findAll('[data-slot="chart-spinner"]').length).toBeGreaterThan(0)
    expect(status.find('.sr-only').text()).toBe('Yükleniyor…')
    expect(wrapper.find('main [data-stat]').exists()).toBe(false)

    resolve(reportOf())
    await settle()
    expect(wrapper.find('main [data-testid="metric-skeleton"]').exists()).toBe(false)
    expect(wrapper.get('main [data-stat="deliveryCost"] dd').text()).toBe('$12,50')
  })

  it('shows live click counts and no CTR before the campaign completes', async () => {
    vi.mocked(analytics.getCampaignSummary).mockResolvedValue(
      reportOf({
        campaign_id: 'c2',
        completed_at: null as unknown as string,
        deliveries: [],
        clicks: {
          sent: 0,
          unique_clicks: 3,
          total_clicks: 5,
          ctr: 0,
          by_channel: [{ channel: 'email', sent: 0, unique_clicks: 3, total_clicks: 5, ctr: 0 }],
          by_locale: [
            {
              channel: 'email',
              language: 'tr',
              timezone: 'Europe/Istanbul',
              sent: 0,
              unique_clicks: 3,
              total_clicks: 5,
              ctr: 0,
            },
          ],
        },
      }),
    )
    const { wrapper } = await open('/analytics/campaigns/c2')
    const section = wrapper.get('main [data-section="clicks"]')

    expect(section.get('[data-testid="ctr-pending"]').text()).toContain('CTR kesinleşir')
    expect(section.find('[data-stat="ctr"]').exists()).toBe(false)
    expect(section.find('[data-stat="ctr-email"]').exists()).toBe(false)
    expect(section.get('[data-stat="uniqueClicks"] dd').text()).toBe('3')
    expect(section.get('[data-stat="totalClicks"] dd').text()).toBe('5')
    // The sent count and the rate are unknown until completion: a dash, never 0 or %0.
    expect(
      section
        .get('[data-row="locale"]')
        .findAll('th, td')
        .map((c) => c.text()),
    ).toEqual(['E-posta', 'Türkçe', 'Europe/Istanbul', '—', '3', '5', '—'])
    expect(section.text()).not.toContain('%0')
  })

  it('says there are no clicks yet for a campaign that has not completed and has none', async () => {
    vi.mocked(analytics.getCampaignSummary).mockResolvedValue(
      reportOf({
        completed_at: null as unknown as string,
        clicks: {
          sent: 0,
          unique_clicks: 0,
          total_clicks: 0,
          ctr: 0,
          by_channel: [],
          by_locale: [],
        },
      }),
    )
    const { wrapper } = await open('/analytics/campaigns/c2')
    const section = wrapper.get('main [data-section="clicks"]')

    expect(section.get('[data-testid="clicks-empty"]').text()).toBe(
      'Henüz tıklama veya gönderim yok.',
    )
    expect(section.find('[data-stat]').exists()).toBe(false)
  })

  it('says Analytics has no data yet when the report is missing', async () => {
    vi.mocked(analytics.getCampaignSummary).mockResolvedValue(null)
    const { wrapper } = await open()
    expect(wrapper.get('main').text()).toContain('Analytics bu kampanya için henüz veri toplamadı.')
    expect(wrapper.find('main [role="alert"]').exists()).toBe(false)
  })

  it('reports a failed request as an error line', async () => {
    vi.mocked(analytics.getCampaignSummary).mockRejectedValue(new TypeError('offline'))
    const { wrapper } = await open()
    expect(wrapper.get('main [role="alert"]').text()).toContain('Analiz verisi alınamadı')
  })

  it('links to the chat of the campaign', async () => {
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf({ id: 'c1' }))
    const { wrapper, router } = await open()
    const link = wrapper.get('main a[href="/campaigns/c1"]')
    expect(link.text()).toBe('Sohbeti aç')
    await link.trigger('click')
    await settle()
    expect(router.currentRoute.value.name).toBe('campaign')
  })
})

describe('CampaignSummary sections', () => {
  const Extra = defineComponent({
    props: { report: { type: Object, required: true } },
    setup: (props) => () =>
      h(
        'p',
        { 'data-testid': 'extra' },
        `kampanya ${(props.report as { campaign_id: string }).campaign_id}`,
      ),
  })

  it('renders each extra section under its title with the report, so a new block needs no change here', async () => {
    const summary = mount(CampaignSummary, {
      props: {
        report: reportOf(),
        sections: [{ key: 'success', title: 'Ek bölüm', component: Extra }],
      },
      global: { plugins: [createAppI18n()] },
    })
    await settle()

    const section = summary.get('[data-section="success"]')
    expect(section.get('h3').text()).toBe('Ek bölüm')
    expect(section.get('[data-testid="extra"]').text()).toBe('kampanya c1')
  })

  it('has no section and no placeholder by default', async () => {
    const summary = mount(CampaignSummary, {
      props: { report: reportOf() },
      global: { plugins: [createAppI18n()] },
    })
    await settle()
    expect(summary.find('[data-section]').exists()).toBe(false)
  })
})
