import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import * as analytics from '@/api/analytics/service'
import * as proposal from '@/api/proposal/service'
import { FakeEventSource, infoOf, mountApp, viewOf } from '@/__tests__/helpers'
import { setViewportWide } from '@/__tests__/setup'
import { APP_MENU, MENU_LINKS } from '@/features/dashboard/menu'
import { resetToasts } from '@/features/campaigns/composables/useToasts'

import { freezeToday, mockDashboardApi, settle } from './harness'

vi.mock('@/api/proposal/service')
vi.mock('@/api/analytics/service')
vi.setConfig({ testTimeout: 20000 })

let wrapper: Awaited<ReturnType<typeof mountApp>>['wrapper'] | undefined

beforeEach(() => {
  FakeEventSource.install()
  mockDashboardApi()
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  resetToasts()
  vi.useRealTimers()
  vi.resetAllMocks()
})

async function open(path: string) {
  const mounted = await mountApp(path)
  await settle()
  wrapper = mounted.wrapper
  return mounted
}

describe('routes', () => {
  it('opens the dashboard at / and the brief box at /campaigns/new', async () => {
    const { wrapper } = await open('/')
    expect(wrapper.get('h2').text()).toBe('Genel bakış')
    expect(wrapper.find('textarea').exists()).toBe(false)
    wrapper.unmount()

    const second = await open('/campaigns/new')
    expect(second.wrapper.find('textarea').exists()).toBe(true)
    expect(second.wrapper.text()).toContain('Yeni kampanya')
  })

  it('sends an unknown path to the dashboard', async () => {
    const { router } = await open('/no/such/page')
    expect(router.currentRoute.value.name).toBe('dashboard')
  })

  it('keeps /campaigns/:id on the chat', async () => {
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf({ id: 'c1' }))
    const { router } = await open('/campaigns/c1')
    expect(router.currentRoute.value.name).toBe('campaign')
  })

  it('has the sections under the dashboard', async () => {
    const { router } = await open('/analytics/llm')
    expect(router.currentRoute.value.name).toBe('analytics-llm')
    await router.push('/analytics/campaigns/abc')
    expect(router.currentRoute.value.params.id).toBe('abc')
    await router.push('/live')
    expect(router.currentRoute.value.name).toBe('live')
  })
})

describe('links', () => {
  it('leads from the logo to the dashboard on both screens', async () => {
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf({ id: 'c1' }))
    const chat = await open('/campaigns/c1')
    const logo = chat.wrapper.get('aside a[aria-label="Ana sayfa"]')
    expect(logo.attributes('href')).toBe('/')
    expect(logo.find('img[src="/logo.png"]').exists()).toBe(true)
    await logo.trigger('click')
    await settle()
    expect(chat.router.currentRoute.value.fullPath).toBe('/')
    expect(chat.wrapper.get('h2').text()).toBe('Genel bakış')

    expect(chat.wrapper.get('aside a[aria-label="Ana sayfa"]').attributes('href')).toBe('/')
  })

  it('leads from the + of the chat sidebar to /campaigns/new', async () => {
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf({ id: 'c1' }))
    const { wrapper, router } = await open('/campaigns/c1')
    const plus = wrapper.get('aside a[aria-label="Yeni kampanya"]')
    expect(plus.attributes('href')).toBe('/campaigns/new')
    await plus.trigger('click')
    await settle()
    expect(router.currentRoute.value.fullPath).toBe('/campaigns/new')
  })

  it('leads from the "Kampanya oluştur" card to /campaigns/new', async () => {
    const { wrapper, router } = await open('/')
    const cards = wrapper.findAll('[data-slot="card"]')
    expect(cards).toHaveLength(1)
    expect(cards[0]!.text()).toContain('Kampanya oluştur')
    expect(cards[0]!.attributes('href')).toBe('/campaigns/new')
    await cards[0]!.trigger('click')
    await settle()
    expect(router.currentRoute.value.fullPath).toBe('/campaigns/new')
    expect(wrapper.find('textarea').exists()).toBe(true)
  })

  it('opens a campaign created from /campaigns/new on its chat page', async () => {
    vi.mocked(proposal.createCampaign).mockResolvedValue(infoOf({ id: 'new-1' }))
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf({ id: 'new-1' }))
    const { wrapper, router } = await open('/campaigns/new')
    await wrapper.get('textarea').setValue('Yeni brief')
    await wrapper.get('form').trigger('submit')
    await settle()
    expect(router.currentRoute.value.fullPath).toBe('/campaigns/new-1')
  })
})

describe('the one sidebar', () => {
  const order = (w: NonNullable<typeof wrapper>) =>
    w.findAll('aside nav > [data-menu]').map((el) => el.attributes('data-menu'))

  it('is a config list: a link, the campaign list and a group of three links', () => {
    expect(APP_MENU.map((entry) => [entry.kind, entry.id])).toEqual([
      ['link', 'overview'],
      ['campaigns', 'campaigns'],
      ['group', 'analytics'],
    ])
    const group = APP_MENU.find((entry) => entry.kind === 'group')!
    expect(group.kind === 'group' && group.children.map((c) => c.route)).toEqual([
      'analytics-campaigns',
      'analytics-llm',
      'live',
    ])
    expect(MENU_LINKS.map((link) => link.id)).toEqual([
      'overview',
      'campaign-analytics',
      'llm',
      'live',
    ])
  })

  it('is the same on the dashboard and on a chat page: Genel bakış, Kampanyalar, Analitikler', async () => {
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf({ id: 'c1' }))
    const dashboard = await open('/')
    expect(dashboard.wrapper.findAll('aside')).toHaveLength(1)
    expect(order(dashboard.wrapper)).toEqual(['overview', 'campaigns', 'analytics'])
    expect(dashboard.wrapper.find('aside a[aria-label="Yeni kampanya"]').exists()).toBe(true)
    dashboard.wrapper.unmount()

    const chat = await open('/campaigns/c1')
    expect(chat.wrapper.findAll('aside')).toHaveLength(1)
    expect(order(chat.wrapper)).toEqual(['overview', 'campaigns', 'analytics'])
    expect(chat.wrapper.find('aside a[aria-label="Yeni kampanya"]').exists()).toBe(true)
    expect(chat.wrapper.find('aside img[src="/logo.png"]').exists()).toBe(true)
  })

  it('labels the three top items and links Genel bakış to /', async () => {
    const { wrapper } = await open('/campaigns/new')
    const overview = wrapper.get('aside a[data-menu="overview"]')
    expect(overview.text()).toBe('Genel bakış')
    expect(overview.attributes('href')).toBe('/')
    expect(
      wrapper
        .get('aside section[data-menu="campaigns"] > [data-slot="accordion"] h3 button')
        .text(),
    ).toBe('Kampanyalar')
    expect(wrapper.get('aside [data-menu="analytics"] h3 button').text()).toBe('Analitikler')
  })

  it('lists the campaigns of the chat in the sidebar on the dashboard too, grouped by status, and opens their chat', async () => {
    vi.mocked(proposal.listCampaigns).mockResolvedValue([
      infoOf({
        id: 'a',
        brief: 'Taslak brief',
        status: 'drafting',
        created_at: '2026-10-29T09:00:00Z',
      }),
    ])
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf({ id: 'a' }))
    const { wrapper, router } = await open('/')

    const row = wrapper.get('aside [data-group="drafting"] li')
    expect(row.text()).toContain('Taslak brief')
    expect(row.text()).toContain('29 Eki 2026')
    expect(row.get('a').attributes('href')).toBe('/campaigns/a')
    await row.get('a').trigger('click')
    await settle()
    expect(router.currentRoute.value.name).toBe('campaign')
  })

  it('has three items under Analitikler: two accordions of campaigns and a plain link for the LLM page', async () => {
    vi.mocked(proposal.listCampaigns).mockResolvedValue([
      infoOf({ id: 'a', brief: 'Gönderimdeki', status: 'sending' }),
      infoOf({ id: 'b', brief: 'Bitmiş', status: 'completed' }),
      infoOf({ id: 'c', brief: 'Taslak', status: 'drafting' }),
    ])
    const { wrapper } = await open('/campaigns/new')
    const group = wrapper.get('aside [data-menu="analytics"]')
    expect(group.get('h3 button').attributes('aria-expanded')).toBe('false')

    await group.get('h3 button').trigger('click')
    await settle()

    const items = group.findAll('li[data-menu]')
    expect(items.map((li) => li.attributes('data-menu'))).toEqual([
      'campaign-analytics',
      'llm',
      'live',
    ])
    const llm = group.get('li[data-menu="llm"] a')
    expect(llm.text()).toBe('LLM analitiği')
    expect(llm.attributes('href')).toBe('/analytics/llm')
    expect(group.find('li[data-menu="llm"] button').exists()).toBe(false)
    expect(group.get('[data-menu="campaign-analytics"] h3 button').text()).toBe(
      'Kampanya analitiği',
    )
    expect(group.get('[data-menu="live"] h3 button').text()).toBe('Canlı gönderim')
  })

  it('opens the campaign accordions on click and lists the campaigns of each: sending and completed for analytics, sending for live', async () => {
    vi.mocked(proposal.listCampaigns).mockResolvedValue([
      infoOf({
        id: 'a',
        brief: 'Gönderimdeki',
        status: 'sending',
        created_at: '2026-10-29T09:00:00Z',
      }),
      infoOf({ id: 'b', brief: 'Bitmiş', status: 'completed' }),
      infoOf({ id: 'c', brief: 'Taslak', status: 'drafting' }),
      infoOf({ id: 'd', brief: 'Zamanlanan', status: 'scheduled' }),
      infoOf({ id: 'e', brief: 'İptal', status: 'cancelled' }),
    ])
    const { wrapper } = await open('/campaigns/new')
    await wrapper.get('aside [data-menu="analytics"] h3 button').trigger('click')
    await settle()
    await wrapper.get('aside [data-menu="campaign-analytics"] h3 button').trigger('click')
    await wrapper.get('aside [data-menu="live"] h3 button').trigger('click')
    await settle()

    const analyticsItem = wrapper.get('aside [data-menu="campaign-analytics"]')
    expect(analyticsItem.findAll('[data-group]').map((g) => g.attributes('data-group'))).toEqual([
      'sending',
      'completed',
    ])
    expect(analyticsItem.findAll('li a').map((a) => a.attributes('href'))).toEqual([
      '/analytics/campaigns/a',
      '/analytics/campaigns/b',
    ])
    const row = analyticsItem.get('[data-group="sending"] li')
    expect(row.text()).toContain('Gönderimdeki')
    expect(row.text()).toContain('29 Eki 2026')
    expect(row.find('[data-slot="badge"]').exists()).toBe(false)
    for (const text of ['Taslak', 'Zamanlanan', 'İptal']) {
      expect(analyticsItem.text()).not.toContain(text)
    }

    // Canlı gönderim: sending first, then scheduled; no completed, draft or cancelled one.
    const live = wrapper.get('aside [data-menu="live"]')
    expect(live.findAll('[data-group]').map((g) => g.attributes('data-group'))).toEqual([
      'sending',
      'scheduled',
    ])
    expect(live.findAll('li a').map((a) => [a.text(), a.attributes('href')])).toEqual([
      [expect.stringContaining('Gönderimdeki'), '/live/a'],
      [expect.stringContaining('Zamanlanan'), '/live/d'],
    ])
    for (const text of ['Bitmiş', 'Taslak', 'İptal']) expect(live.text()).not.toContain(text)
  })

  it('hides an empty status group and says so in a line when a menu item has no campaign', async () => {
    vi.mocked(proposal.listCampaigns).mockResolvedValue([
      infoOf({ id: 'b', brief: 'Bitmiş', status: 'completed' }),
      infoOf({ id: 'c', brief: 'Taslak', status: 'drafting' }),
    ])
    const { wrapper } = await open('/analytics/llm')
    await wrapper.get('aside [data-menu="campaign-analytics"] h3 button').trigger('click')
    await wrapper.get('aside [data-menu="live"] h3 button').trigger('click')
    await settle()

    expect(
      wrapper
        .findAll('aside [data-menu="campaign-analytics"] [data-group]')
        .map((g) => g.attributes('data-group')),
    ).toEqual(['completed'])
    expect(wrapper.get('aside [data-menu="live"]').text()).toContain(
      'Şu anda gönderimde veya zamanlanmış kampanya yok.',
    )
  })

  it('says there is no started campaign in a line when only drafts exist', async () => {
    vi.mocked(proposal.listCampaigns).mockResolvedValue([infoOf({ id: 'c', status: 'drafting' })])
    const { wrapper } = await open('/analytics/llm')
    await wrapper.get('aside [data-menu="campaign-analytics"] h3 button').trigger('click')
    await settle()
    const item = wrapper.get('aside [data-menu="campaign-analytics"]')
    expect(item.text()).toContain('Gönderimi başlamış veya tamamlanmış kampanya yok.')
    expect(item.find('[data-group]').exists()).toBe(false)
  })

  it('unfolds the groups of the open campaign page and highlights its row', async () => {
    vi.mocked(proposal.listCampaigns).mockResolvedValue([
      infoOf({ id: 'a', brief: 'Gönderimdeki', status: 'sending' }),
      infoOf({ id: 'b', brief: 'Bitmiş', status: 'completed' }),
    ])
    const { wrapper } = await open('/analytics/campaigns/b')

    expect(wrapper.get('aside [data-menu="analytics"] h3 button').attributes('aria-expanded')).toBe(
      'true',
    )
    const item = wrapper.get('aside [data-menu="campaign-analytics"]')
    expect(item.get('h3 button').attributes('aria-expanded')).toBe('true')
    expect(item.get('[data-group="completed"] button').attributes('aria-expanded')).toBe('true')
    const current = wrapper.findAll('aside nav [aria-current="page"]')
    expect(current.map((a) => a.text())).toEqual([expect.stringContaining('Bitmiş')])
    expect(current[0]!.classes()).toContain('bg-accent')
    // The other item stays folded.
    expect(wrapper.get('aside [data-menu="live"] h3 button').attributes('aria-expanded')).toBe(
      'false',
    )
  })

  it('highlights the open page and unfolds Analitikler for it', async () => {
    const { wrapper } = await open('/analytics/llm')
    const group = wrapper.get('aside [data-menu="analytics"]')
    expect(group.get('h3 button').attributes('aria-expanded')).toBe('true')
    const current = wrapper.findAll('aside nav [aria-current="page"]')
    expect(current.map((a) => a.text())).toEqual(['LLM analitiği'])
    expect(current[0]!.classes()).toContain('bg-accent')
    wrapper.unmount()

    const home = await open('/')
    expect(home.wrapper.findAll('aside nav [aria-current="page"]').map((a) => a.text())).toEqual([
      'Genel bakış',
    ])
  })

  it('keeps the date range in the links of the pages that have the filter', async () => {
    freezeToday()
    const { wrapper } = await open('/analytics/llm?from=2026-10-01&to=2026-10-30')
    expect(wrapper.get('aside li[data-menu="llm"] a').attributes('href')).toBe(
      '/analytics/llm?from=2026-10-01&to=2026-10-30',
    )
    expect(wrapper.get('aside a[data-menu="overview"]').attributes('href')).toBe(
      '/?from=2026-10-01&to=2026-10-30',
    )
  })

  it('has no analytics summary box', async () => {
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf({ id: 'c1' }))
    const { wrapper } = await open('/campaigns/c1')
    expect(wrapper.find('aside section[aria-label="Analiz"]').exists()).toBe(false)
    expect(wrapper.find('aside dl').exists()).toBe(false)
  })
})

describe('narrow screen', () => {
  it('has the top bar with the menu button and a logo that leads to the dashboard, and the one menu in a drawer', async () => {
    setViewportWide(false)
    const { wrapper } = await open('/')
    expect(wrapper.find('aside').exists()).toBe(false)
    const bar = wrapper.get('header')
    expect(bar.get('button').attributes('aria-label')).toBe('Menüyü aç')
    expect(bar.get('a').attributes('href')).toBe('/')
    expect(bar.get('img').attributes('src')).toBe('/logo.png')

    await bar.get('button').trigger('click')
    await settle()
    const drawer = document.body.querySelector('[role="dialog"]')!
    expect(
      [...drawer.querySelectorAll('nav > [data-menu]')].map((el) => el.getAttribute('data-menu')),
    ).toEqual(['overview', 'campaigns', 'analytics'])
  })
})

describe('Analytics calls', () => {
  it('are not made on the chat screen', async () => {
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf({ id: 'c1' }))
    await open('/campaigns/c1')
    expect(analytics.getOverview).not.toHaveBeenCalled()
  })
})
