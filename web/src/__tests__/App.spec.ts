import { flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import * as analytics from '@/api/analytics/service'
import * as proposal from '@/api/proposal/service'
import { resetToasts } from '@/features/campaigns/composables/useToasts'

import { FakeEventSource, infoOf, mountApp, viewOf } from './helpers'
import { setViewportWide } from './setup'

vi.mock('@/api/proposal/service')
vi.mock('@/api/analytics/service')

let wrapper: Awaited<ReturnType<typeof mountApp>>['wrapper'] | undefined

beforeEach(() => {
  FakeEventSource.install()
  vi.mocked(proposal.listCampaigns).mockResolvedValue([])
  vi.mocked(analytics.getCampaignSummary).mockResolvedValue(null)
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  resetToasts()
  vi.resetAllMocks()
})

async function open(path = '/campaigns/new') {
  const mounted = await mountApp(path)
  wrapper = mounted.wrapper
  return mounted
}

describe('app shell', () => {
  it('draws the logo and a + for a new campaign in the sidebar, with no top header, profile block, search or filter', async () => {
    const { wrapper } = await open()

    expect(wrapper.find('header').exists()).toBe(false)
    expect(wrapper.find('h1').exists()).toBe(false)
    const logo = wrapper.get('aside img[src="/logo.png"]')
    expect(logo.attributes('alt')).toBe('coldbru')
    const plus = wrapper.get('aside a[aria-label="Yeni kampanya"]')
    expect(plus.attributes('href')).toBe('/campaigns/new')
    expect(plus.find('svg').exists()).toBe(true)
    expect(plus.text()).toBe('')
    expect(wrapper.find('input[type="search"]').exists()).toBe(false)
    expect(wrapper.find('aside select').exists()).toBe(false)
    expect(wrapper.find('[data-mode]').exists()).toBe(false)
    expect(wrapper.text()).not.toMatch(/profil/i)
  })

  it('on a narrow screen has a top bar with the menu button and the logo, and the list in a drawer', async () => {
    setViewportWide(false)
    const { wrapper } = await open()

    expect(wrapper.find('aside').exists()).toBe(false)
    const bar = wrapper.get('header')
    expect(bar.get('button').attributes('aria-label')).toBe('Menüyü aç')
    expect(bar.get('img').attributes('src')).toBe('/logo.png')
    expect(bar.get('img').attributes('alt')).toBe('coldbru')
    expect(wrapper.find('h1').exists()).toBe(false)
  })

  it('shows the empty state with the brief box on /campaigns/new', async () => {
    const { wrapper } = await open('/campaigns/new')

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
      id: 'b2',
      brief: 'Bursa kış kampanyası',
      status: 'drafting',
      created_at: '2026-10-27T09:00:00Z',
    }),
    infoOf({
      id: 'c',
      brief: 'İzmir hafta sonu kampanyası',
      status: 'cancelled',
      created_at: '2026-10-28T09:00:00Z',
    }),
  ]

  const groups = (wrapper: Awaited<ReturnType<typeof open>>['wrapper']) =>
    wrapper.findAll('aside [data-group]')
  const groupTrigger = (wrapper: Awaited<ReturnType<typeof open>>['wrapper'], status: string) =>
    wrapper.get(`aside [data-group="${status}"] button`)

  it('wraps the groups in a "Kampanyalar" accordion that folds and unfolds', async () => {
    vi.mocked(proposal.listCampaigns).mockResolvedValue(campaigns)
    const { wrapper } = await open()

    const root = wrapper.get(
      'aside section > [data-slot="accordion"] > [data-slot="accordion-item"] > h3 button',
    )
    expect(root.text()).toBe('Kampanyalar')
    expect(root.attributes('aria-expanded')).toBe('true')
    expect(groups(wrapper)).toHaveLength(3)

    await root.trigger('click')
    expect(root.attributes('aria-expanded')).toBe('false')
    expect(groups(wrapper)).toHaveLength(0)

    await root.trigger('click')
    expect(groups(wrapper)).toHaveLength(3)
  })

  it('sizes the "Kampanyalar" block to its content, so "Analitikler" sits right below it when folded', async () => {
    vi.mocked(proposal.listCampaigns).mockResolvedValue(campaigns)
    const { wrapper } = await open()

    // Not flex-1: a folded accordion must not keep the free height of the sidebar. It may shrink and
    // scroll (flex: 0 1 auto), so the menu entries around it stay on screen.
    const section = wrapper.get('aside nav > section[data-menu="campaigns"]')
    expect(section.classes()).toContain('flex-initial')
    expect(section.classes()).not.toContain('flex-1')
    expect(section.classes()).toContain('overflow-y-auto')
    expect(section.get('[data-slot="accordion"]').classes()).not.toContain('flex-1')
    expect(section.get('[data-slot="accordion"]').classes()).not.toContain('shrink-0')
  })

  it('groups the campaigns by status in a fixed order and hides the empty groups', async () => {
    vi.mocked(proposal.listCampaigns).mockResolvedValue(campaigns)
    const { wrapper } = await open()

    // Scheduled and sending have no campaign: no group.
    expect(groups(wrapper).map((g) => g.attributes('data-group'))).toEqual([
      'drafting',
      'completed',
      'cancelled',
    ])
    expect(groups(wrapper).map((g) => g.get('button').text())).toEqual([
      'Taslak 2',
      'Tamamlandı 1',
      'İptal 1',
    ])
    expect(proposal.listCampaigns).toHaveBeenCalledWith(0)
  })

  it('names all five groups', async () => {
    vi.mocked(proposal.listCampaigns).mockResolvedValue(
      (['cancelled', 'sending', 'completed', 'scheduled', 'drafting'] as const).map((status) =>
        infoOf({ id: status, status }),
      ),
    )
    const { wrapper } = await open()

    expect(
      groups(wrapper).map((g) =>
        g
          .get('button')
          .text()
          .replace(/\s*\d+$/, ''),
      ),
    ).toEqual(['Taslak', 'Zamanlanmış', 'Gönderiliyor', 'Tamamlandı', 'İptal'])
  })

  it('opens the drafts and keeps the other groups folded when no campaign is open', async () => {
    vi.mocked(proposal.listCampaigns).mockResolvedValue(campaigns)
    const { wrapper } = await open()

    expect(groupTrigger(wrapper, 'drafting').attributes('aria-expanded')).toBe('true')
    expect(groupTrigger(wrapper, 'completed').attributes('aria-expanded')).toBe('false')
    expect(groupTrigger(wrapper, 'cancelled').attributes('aria-expanded')).toBe('false')
    expect(wrapper.findAll('aside li').map((r) => r.text())).toEqual([
      expect.stringContaining('Ankara yaz indirimi'),
      expect.stringContaining('Bursa kış kampanyası'),
    ])

    await groupTrigger(wrapper, 'completed').trigger('click')
    expect(wrapper.findAll('aside li')).toHaveLength(3)
    expect(wrapper.get('aside [data-group="completed"] li').text()).toContain(
      'İstanbul sepet hatırlatması',
    )
  })

  it('also opens the group of the campaign that is open', async () => {
    vi.mocked(proposal.listCampaigns).mockResolvedValue(campaigns)
    vi.mocked(proposal.getCampaign).mockResolvedValue(
      viewOf({ id: 'c', status: 'cancelled', brief: 'İzmir hafta sonu kampanyası' }),
    )
    const { wrapper } = await open('/campaigns/c')

    expect(groupTrigger(wrapper, 'drafting').attributes('aria-expanded')).toBe('true')
    expect(groupTrigger(wrapper, 'cancelled').attributes('aria-expanded')).toBe('true')
    expect(groupTrigger(wrapper, 'completed').attributes('aria-expanded')).toBe('false')
    const selected = wrapper.get('aside a[aria-current="page"]')
    expect(selected.text()).toContain('İzmir hafta sonu kampanyası')
  })

  it('shows only the title and the short date in a row, with no badge or card', async () => {
    vi.mocked(proposal.listCampaigns).mockResolvedValue(campaigns)
    const { wrapper } = await open()

    const row = wrapper.get('aside [data-group="drafting"] li')
    expect(row.findAll('a > *').map((el) => el.text())).toEqual([
      'Ankara yaz indirimi',
      '29 Eki 2026',
    ])
    expect(row.find('time').attributes('datetime')).toBe('2026-10-29T09:00:00Z')
    expect(wrapper.find('aside li [data-slot="badge"]').exists()).toBe(false)
    expect(wrapper.find('aside li [data-status]').exists()).toBe(false)
    expect(
      wrapper.findAll('aside li').every((li) => !li.text().match(/Taslak|Tamamlandı|İptal/)),
    ).toBe(true)
    // The title and the date share one line: both are children of the same link.
    const link = row.get('a')
    expect(link.classes()).toContain('flex')
    expect(link.classes()).not.toContain('border')
    // The accordion content underlines links; the row must win against it.
    expect(link.classes()).toContain('no-underline!')
    expect(link.findAll('span, time')).toHaveLength(2)
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

  it('loads the next page from the offset of what the server already handed out, after the groups', async () => {
    const page = Array.from({ length: 20 }, (_, i) =>
      infoOf({ id: `p1-${i}`, brief: `Brief ${i}` }),
    )
    const second = [infoOf({ id: 'p2-0', brief: 'Brief son' })]
    vi.mocked(proposal.listCampaigns).mockResolvedValueOnce(page).mockResolvedValueOnce(second)
    const { wrapper } = await open()

    const more = wrapper.findAll('button').find((b) => b.text() === 'Daha fazla yükle')!
    // The button follows the last group.
    expect(
      wrapper.get('aside [data-group="drafting"]').element.compareDocumentPosition(more.element) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy()
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
    expect(wrapper.find('aside [role="alert"]').exists()).toBe(true)

    vi.mocked(proposal.listCampaigns).mockResolvedValue(campaigns)
    await wrapper
      .findAll('button')
      .find((b) => b.text() === 'Yeniden dene')!
      .trigger('click')
    await flushPromises()

    expect(groups(wrapper)).toHaveLength(3)
  })
})

describe('single sidebar (D-099)', () => {
  it('has no analytics summary box, whichever campaign is open', async () => {
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf({ id: 'c1' }))
    const { wrapper } = await open('/campaigns/c1')

    expect(wrapper.find('section[aria-label="Analiz"]').exists()).toBe(false)
    expect(analytics.getCampaignSummary).not.toHaveBeenCalled()
  })
})
