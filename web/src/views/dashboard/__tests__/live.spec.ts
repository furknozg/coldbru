import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import * as analytics from '@/api/analytics/service'
import * as proposal from '@/api/proposal/service'
import { FakeEventSource, infoOf, mountApp } from '@/__tests__/helpers'
import { resetToasts } from '@/features/campaigns/composables/useToasts'

import { cardTitles, mockDashboardApi, settle } from './harness'

vi.mock('@/api/proposal/service')
vi.mock('@/api/analytics/service')
vi.setConfig({ testTimeout: 20000 })

let wrapper: Awaited<ReturnType<typeof mountApp>>['wrapper'] | undefined

beforeEach(() => {
  FakeEventSource.install()
  mockDashboardApi()
  vi.mocked(proposal.listCampaigns).mockResolvedValue([
    infoOf({ id: 'c1', brief: 'Gönderimdeki kampanya', status: 'sending' }),
    infoOf({ id: 'c2', brief: 'Zamanlanan kampanya', status: 'scheduled' }),
  ])
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  resetToasts()
  vi.resetAllMocks()
})

async function open(path = '/live/c1') {
  const mounted = await mountApp(path)
  await settle()
  wrapper = mounted.wrapper
  return mounted
}

const snapshot = (over: Record<string, unknown> = {}) => ({
  campaign_id: 'c1',
  status: 'sending',
  total: 200,
  updated_at: '2026-10-30T10:00:00Z',
  counts: [
    { channel: 'email', status: 'sent', count: 60 },
    { channel: 'email', status: 'pending', count: 40 },
    { channel: 'sms', status: 'sent', count: 20 },
    { channel: 'sms', status: 'failed', count: 10 },
    { channel: 'sms', status: 'skipped_no_consent', count: 10 },
    { channel: 'sms', status: 'pending', count: 60 },
  ],
  ...over,
})

const cell = (w: NonNullable<typeof wrapper>, channel: string, status: string) =>
  w.get(`tr[data-channel="${channel}"] td[data-status="${status}"]`).text()
const bar = (w: NonNullable<typeof wrapper>) => w.get('[role="progressbar"]')

describe('Canlı gönderim', () => {
  it('opens the delivery stream of the chosen campaign and waits for its first snapshot', async () => {
    const { wrapper } = await open()

    expect(FakeEventSource.open.map((es) => es.url)).toEqual([
      '/api/notification/campaigns/c1/deliveries/events',
    ])
    expect(wrapper.get('[data-testid="campaign-title"]').text()).toBe('Gönderimdeki kampanya')
    expect(wrapper.get('main').text()).toContain('Gönderim durumu bekleniyor…')
    expect(wrapper.get('[data-testid="connection"]').attributes('data-state')).toBe('connecting')
  })

  it('shows the counts per channel and status and the progress of the finished deliveries', async () => {
    const { wrapper } = await open()
    const es = FakeEventSource.last
    es.onopen?.()
    es.emit('snapshot', snapshot())
    await settle()

    expect(wrapper.get('[data-testid="connection"]').text()).toBe('Canlı akış açık')
    expect(cell(wrapper, 'email', 'sent')).toBe('60')
    expect(cell(wrapper, 'email', 'pending')).toBe('40')
    expect(cell(wrapper, 'email', 'failed')).toBe('0')
    expect(cell(wrapper, 'sms', 'failed')).toBe('10')
    expect(cell(wrapper, 'sms', 'skipped_no_consent')).toBe('10')
    expect(wrapper.findAll('thead th').map((th) => th.text())).toEqual([
      'Kanal',
      'Bekleyen',
      'Gönderiliyor',
      'Gönderildi',
      'Başarısız',
      'İzin yok',
      'İptal',
    ])
    // sent 80 + failed 10 + skipped 10 = 100 of 200.
    expect(bar(wrapper).attributes('aria-valuenow')).toBe('50')
    expect(wrapper.get('main').text()).toContain('100 / 200 teslimat sonuçlandı')
    expect(wrapper.get('main').text()).toContain('%50')
  })

  it('titles the progress card and the table card', async () => {
    const { wrapper } = await open()
    FakeEventSource.last.emit('snapshot', snapshot())
    await settle()

    expect(cardTitles(wrapper.get('main'))).toEqual(['Gönderim ilerlemesi', 'Teslimatlar'])
    expect(wrapper.get('[role="progressbar"]').element.closest('[data-slot="metric-card"]')).toBe(
      wrapper.findAll('main [data-slot="metric-card"]')[0]!.element,
    )
  })

  it('follows every new snapshot', async () => {
    const { wrapper } = await open()
    const es = FakeEventSource.last
    es.emit('snapshot', snapshot())
    await settle()

    es.emit(
      'snapshot',
      snapshot({
        counts: [
          { channel: 'email', status: 'sent', count: 100 },
          { channel: 'sms', status: 'sent', count: 80 },
          { channel: 'sms', status: 'failed', count: 20 },
        ],
      }),
    )
    await settle()

    expect(cell(wrapper, 'email', 'sent')).toBe('100')
    expect(cell(wrapper, 'email', 'pending')).toBe('0')
    expect(bar(wrapper).attributes('aria-valuenow')).toBe('100')
  })

  it('says the sending is over and closes the stream on the done event', async () => {
    const { wrapper } = await open()
    const es = FakeEventSource.last
    es.onopen?.()
    es.emit('snapshot', snapshot())
    es.emit('done', { status: 'completed' })
    await settle()

    expect(es.closed).toBe(true)
    expect(wrapper.get('[data-testid="connection"]').text()).toBe('Gönderim tamamlandı')
    // The last counts stay on the screen.
    expect(cell(wrapper, 'email', 'sent')).toBe('60')
  })

  it('says the campaign was cancelled', async () => {
    const { wrapper } = await open()
    FakeEventSource.last.emit('done', { status: 'cancelled' })
    await settle()
    expect(wrapper.get('[data-testid="connection"]').text()).toBe('Kampanya iptal edildi')
  })

  it('says when the live connection drops', async () => {
    const { wrapper } = await open()
    FakeEventSource.last.onerror?.()
    await settle()
    expect(wrapper.get('[data-testid="connection"]').text()).toBe('Canlı akış bağlantısı koptu')
  })

  it('shows 0% for a campaign without deliveries', async () => {
    const { wrapper } = await open()
    FakeEventSource.last.emit('snapshot', snapshot({ total: 0, counts: [] }))
    await settle()
    expect(bar(wrapper).attributes('aria-valuenow')).toBe('0')
  })

  it('closes the stream when the page is left', async () => {
    const { router } = await open()
    const es = FakeEventSource.last
    expect(es.closed).toBe(false)

    await router.push('/')
    await settle()

    expect(es.closed).toBe(true)
    expect(FakeEventSource.open).toHaveLength(0)
  })

  it('closes the stream of one campaign and opens the next when another is picked', async () => {
    const { router, wrapper } = await open()
    const first = FakeEventSource.last
    first.emit('snapshot', snapshot())
    await settle()
    expect(cell(wrapper, 'email', 'sent')).toBe('60')

    await router.push('/live/c2')
    await settle()

    expect(first.closed).toBe(true)
    expect(FakeEventSource.open.map((es) => es.url)).toEqual([
      '/api/notification/campaigns/c2/deliveries/events',
    ])
    // The counts of the first campaign are gone until the second one reports.
    expect(wrapper.find('tr[data-channel]').exists()).toBe(false)
    expect(wrapper.get('main').text()).toContain('Gönderim durumu bekleniyor…')
  })

  it('lists the sending and the scheduled campaigns in two groups in the menu, not in the content area', async () => {
    vi.mocked(proposal.listCampaigns).mockResolvedValue([
      infoOf({ id: 'a', brief: 'Gönderimdeki', status: 'sending' }),
      infoOf({ id: 'b', brief: 'Zamanlanan', status: 'scheduled' }),
      infoOf({ id: 'c', brief: 'Bitmiş', status: 'completed' }),
    ])
    const { wrapper } = await open('/live')
    const item = wrapper.get('aside [data-menu="live"]')

    expect(item.get('h3 button').attributes('aria-expanded')).toBe('true')
    expect(item.findAll('[data-group]').map((g) => g.attributes('data-group'))).toEqual([
      'sending',
      'scheduled',
    ])
    expect(item.findAll('li a').map((a) => a.attributes('href'))).toEqual(['/live/a', '/live/b'])
    expect(item.text()).not.toContain('Bitmiş')
    expect(wrapper.find('main [data-group]').exists()).toBe(false)
  })

  it('hides the group that has no campaign', async () => {
    vi.mocked(proposal.listCampaigns).mockResolvedValue([
      infoOf({ id: 'b', brief: 'Zamanlanan', status: 'scheduled' }),
    ])
    const { wrapper } = await open('/live')
    expect(
      wrapper
        .findAll('aside [data-menu="live"] [data-group]')
        .map((g) => g.attributes('data-group')),
    ).toEqual(['scheduled'])
  })

  it('follows a scheduled campaign like any other: the stream opens and counts flow once sending begins', async () => {
    vi.mocked(proposal.listCampaigns).mockResolvedValue([
      infoOf({ id: 'b', brief: 'Zamanlanan', status: 'scheduled' }),
    ])
    const { wrapper } = await open('/live/b')
    expect(FakeEventSource.last.url).toBe('/api/notification/campaigns/b/deliveries/events')
    FakeEventSource.last.emit('snapshot', snapshot({ campaign_id: 'b', status: 'scheduled' }))
    await settle()
    expect(cell(wrapper, 'email', 'pending')).toBe('40')
    FakeEventSource.last.emit('snapshot', snapshot({ campaign_id: 'b', status: 'sending' }))
    await settle()
    expect(cell(wrapper, 'email', 'sent')).toBe('60')
  })

  it('highlights the campaign that is followed', async () => {
    const { wrapper } = await open('/live/c1')
    expect(wrapper.get('aside nav a[aria-current="page"]').text()).toContain(
      'Gönderimdeki kampanya',
    )
  })

  it('opens no stream while no campaign is picked, and asks for one', async () => {
    const { wrapper } = await open('/live')
    expect(FakeEventSource.instances).toHaveLength(0)
    expect(wrapper.get('main').text()).toContain('Menüden izlemek istediğiniz kampanyayı seçin.')
  })

  it('says nothing is being sent when no campaign is scheduled or sending', async () => {
    vi.mocked(proposal.listCampaigns).mockResolvedValue([infoOf({ id: 'x', status: 'completed' })])
    const { wrapper } = await open('/live')
    expect(wrapper.get('main').text()).toContain(
      'Şu anda gönderimde veya zamanlanmış kampanya yok.',
    )
    expect(FakeEventSource.instances).toHaveLength(0)
  })

  it('uses no Analytics call', async () => {
    await open()
    expect(analytics.getCampaignSummary).not.toHaveBeenCalled()
    expect(analytics.getOverview).not.toHaveBeenCalled()
  })
})
