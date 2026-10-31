import { CalendarDate } from '@internationalized/date'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import * as analytics from '@/api/analytics/service'
import { FakeEventSource, mountApp } from '@/__tests__/helpers'
import { RangeCalendar } from '@/components/ui/range-calendar'
import { overviewOf } from '@/features/dashboard/__tests__/fixtures'
import { resetToasts } from '@/features/campaigns/composables/useToasts'
import { ApiError } from '@/api/errors'

import { cardTitles, chart, freezeToday, legend, mockDashboardApi, settle, utc } from './harness'

vi.mock('@/api/proposal/service')
vi.mock('@/api/analytics/service')
vi.setConfig({ testTimeout: 20000 })

let wrapper: Awaited<ReturnType<typeof mountApp>>['wrapper'] | undefined

beforeEach(() => {
  FakeEventSource.install()
  mockDashboardApi()
  freezeToday()
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  resetToasts()
  vi.useRealTimers()
  vi.resetAllMocks()
})

async function open(path = '/') {
  const mounted = await mountApp(path)
  await settle()
  wrapper = mounted.wrapper
  return mounted
}

const main = (w: NonNullable<typeof wrapper>) => w.get('main')
const stat = (w: NonNullable<typeof wrapper>, key: string) => ({
  label: w.get(`[data-stat="${key}"] dt`).text(),
  value: w.get(`[data-stat="${key}"] dd`).text(),
})
const button = (w: NonNullable<typeof wrapper>, text: string) =>
  w.findAll('main button').find((b) => b.text() === text)!

describe('Genel bakış', () => {
  it('shows the figures of the overview', async () => {
    const { wrapper } = await open()

    expect(stat(wrapper, 'campaigns')).toEqual({ label: 'Tamamlanan kampanya', value: '12' })
    expect(stat(wrapper, 'sent')).toEqual({ label: 'Gönderilen', value: '1.200' })
    expect(stat(wrapper, 'failed')).toEqual({ label: 'Başarısız', value: '34' })
    expect(stat(wrapper, 'deliveryCost').value).toBe('$96,00')
    expect(stat(wrapper, 'llmCalls').value).toBe('56')
    expect(stat(wrapper, 'llmCost').value).toBe('$0,012345')
    expect(stat(wrapper, 'tokens').value).toBe('120.000')
    expect(wrapper.get('[data-stat="tokens"]').text()).toContain('giriş 100.000 · çıkış 20.000')
    // Cache tokens are a tile of their own; the token tile stays input + output.
    expect(stat(wrapper, 'cache')).toEqual({ label: 'Önbellek tokenı', value: '350.000' })
    expect(wrapper.get('[data-stat="cache"]').text()).toContain('okuma 300.000 · yazma 50.000')
  })

  it('shows skeletons with spinning chart areas while the figures and the series load, and keeps the old figures during a refetch (D-111)', async () => {
    let resolve!: (value: ReturnType<typeof overviewOf>) => void
    vi.mocked(analytics.getOverview).mockReturnValue(
      new Promise((r) => {
        resolve = r
      }),
    )
    const { wrapper } = await open()

    const skeletons = wrapper.findAll('main [data-testid="metric-skeleton"]')
    expect(skeletons.length).toBeGreaterThan(0)
    expect(main(wrapper).findAll('[data-slot="skeleton"]').length).toBeGreaterThan(0)
    expect(main(wrapper).findAll('[data-slot="chart-spinner"]').length).toBeGreaterThan(0)
    expect(wrapper.find('main [data-stat]').exists()).toBe(false)
    // The series came back already: its charts are there, in cards, with the spinner gone.
    expect(wrapper.findAll('main figure').length).toBeGreaterThan(0)

    resolve(overviewOf())
    await settle()
    expect(stat(wrapper, 'sent').value).toBe('1.200')
    expect(
      wrapper.get('[data-stat="sent"]').element.closest('[data-slot="metric-card"]'),
    ).not.toBeNull()

    // A new range: the old figures stay in place until the new ones arrive, no skeleton.
    vi.mocked(analytics.getOverview).mockReturnValue(new Promise(() => {}))
    await button(wrapper, 'Son 7 gün').trigger('click')
    await settle()
    expect(stat(wrapper, 'sent').value).toBe('1.200')
    expect(wrapper.find('main [data-testid="metric-skeleton"]').exists()).toBe(false)
  })

  it('shows the CTR as a tile next to the others, with unique and total clicks', async () => {
    const { wrapper } = await open()

    expect(stat(wrapper, 'ctr')).toEqual({ label: 'Tıklama oranı (CTR)', value: '%12,5' })
    expect(wrapper.get('[data-stat="ctr"]').text()).toContain('tekil 150 · toplam 210')
    expect(wrapper.get('[data-stat="ctr"]').classes()).not.toContain('border')
  })

  it('shows a dash, not %0, when nothing was sent in the range', async () => {
    vi.mocked(analytics.getOverview).mockResolvedValue(
      overviewOf({
        clicks: { sent: 0, unique_clicks: 0, total_clicks: 0, ctr: 0, by_channel: [] },
      }),
    )
    const { wrapper } = await open()
    expect(stat(wrapper, 'ctr').value).toBe('—')
  })

  it('draws the unique clicks of both channels as a second series over the same buckets', async () => {
    const { wrapper } = await open()

    const clicks = chart(wrapper, 'Kanal başına tekil tıklama · son 90 gün')
    expect(clicks.get('[data-testid="xy-container"]').attributes('data-rows')).toBe('3')
    expect(clicks.get('[data-testid="line"]').attributes('data-series')).toBe('2')
    // 4 + 3 unique e-mail clicks and 2 SMS clicks; the total clicks are not drawn.
    expect(clicks.get('[role="img"]').attributes('aria-label')).toContain('E-posta: 7, SMS: 2')
    expect(clicks.findAll('li').map((li) => li.text())).toEqual(['E-posta', 'SMS'])
  })

  it('titles every card, and a chart has its title as the card heading only, not as a caption too', async () => {
    const { wrapper } = await open()

    expect(cardTitles(main(wrapper))).toEqual([
      'Genel metrikler',
      'Teslimat dağılımı',
      'Kanal başına gönderilen teslimat · son 90 gün',
      'Kanal başına tekil tıklama · son 90 gün',
    ])
    const captions = main(wrapper)
      .findAll('figcaption')
      .map((c) => c.text())
    expect(captions).toEqual(['Kanala göre teslimat', 'Duruma göre teslimat'])
    expect(
      main(wrapper)
        .findAll('h3')
        .map((h) => h.text()),
    ).toEqual(cardTitles(main(wrapper)))
  })

  it('drops the 90-day note from the series titles when the range has a start', async () => {
    const { wrapper } = await open('/?from=2026-10-24&to=2026-10-30')
    expect(cardTitles(main(wrapper)).slice(2)).toEqual([
      'Kanal başına gönderilen teslimat',
      'Kanal başına tekil tıklama',
    ])
  })

  it('scrolls the page inside the dashboard frame, which contains its overscroll', async () => {
    const { wrapper } = await open()
    const frame = main(wrapper).get(':scope > div')
    expect(frame.classes()).toContain('overflow-y-auto')
    expect(frame.classes()).toContain('overscroll-contain')
  })

  it('draws no box around a figure and has the create card as its only card', async () => {
    const { wrapper } = await open()
    expect(wrapper.findAll('[data-slot="card"]')).toHaveLength(1)
    expect(wrapper.get('[data-stat="sent"]').classes()).not.toContain('border')
  })

  it('splits the deliveries by channel and by status in two pies', async () => {
    const { wrapper } = await open()

    expect(legend(wrapper, 'Kanala göre teslimat')).toEqual([
      ['E-posta', '830', '%63,8'],
      ['SMS', '470', '%36,2'],
    ])
    expect(legend(wrapper, 'Duruma göre teslimat').map((row) => row.slice(0, 2))).toEqual([
      ['Gönderildi', '1.200'],
      ['Başarısız', '34'],
      ['İzin yok', '66'],
    ])
  })

  it('draws the sent deliveries of both channels over the buckets of the series', async () => {
    const { wrapper } = await open()

    const sent = chart(wrapper, 'Kanal başına gönderilen teslimat · son 90 gün')
    expect(sent.get('[data-testid="xy-container"]').attributes('data-rows')).toBe('3')
    expect(sent.get('[data-testid="line"]').attributes('data-series')).toBe('2')
    // Zero-filled: 15 e-mails (10 + 5) and 6 SMS over three days, the failed ones not counted.
    expect(sent.get('[role="img"]').attributes('aria-label')).toContain('E-posta: 15, SMS: 6')
    expect(sent.findAll('li').map((li) => li.text())).toEqual(['E-posta', 'SMS'])
  })

  it('says what is wrong in a plain line when the overview fails, and retries', async () => {
    vi.mocked(analytics.getOverview).mockRejectedValueOnce(
      ApiError.from(500, { code: 'internal', title: 'Internal', detail: 'Rapor alınamadı.' }),
    )
    const { wrapper } = await open()

    const alert = main(wrapper).get('[role="alert"]')
    expect(alert.text()).toContain('Rapor alınamadı.')
    expect(wrapper.find('[data-stat]').exists()).toBe(false)

    await alert.get('button').trigger('click')
    await settle()
    expect(stat(wrapper, 'campaigns').value).toBe('12')
  })

  it('shows empty pies as text when nothing was delivered', async () => {
    vi.mocked(analytics.getOverview).mockResolvedValue(overviewOf({ deliveries: [] }))
    const { wrapper } = await open()
    expect(chart(wrapper, 'Duruma göre teslimat').text()).toContain('Gösterilecek veri yok')
  })
})

describe('date filter', () => {
  it('starts on "Tümü": no range for the totals, the last 90 days by day for the series', async () => {
    const { wrapper, router } = await open()

    expect(button(wrapper, 'Tümü').attributes('aria-pressed')).toBe('true')
    expect(analytics.getOverview).toHaveBeenCalledWith({})
    expect(analytics.getOverviewSeries).toHaveBeenCalledWith({
      from: utc(2026, 8, 2),
      to: utc(2026, 10, 31),
      bucket: 'day',
    })
    expect(router.currentRoute.value.query).toEqual({})
  })

  it.each([
    [
      'Bugün',
      { from: '2026-10-30', to: '2026-10-30' },
      [utc(2026, 10, 30), utc(2026, 10, 31)],
      'hour',
    ],
    [
      'Son 7 gün',
      { from: '2026-10-24', to: '2026-10-30' },
      [utc(2026, 10, 24), utc(2026, 10, 31)],
      'day',
    ],
    [
      'Son 30 gün',
      { from: '2026-10-01', to: '2026-10-30' },
      [utc(2026, 10, 1), utc(2026, 10, 31)],
      'day',
    ],
    [
      'Son 90 gün',
      { from: '2026-08-02', to: '2026-10-30' },
      [utc(2026, 8, 2), utc(2026, 10, 31)],
      'day',
    ],
  ] as const)(
    'puts %s in the URL and sends it to both endpoints',
    async (name, query, [from, to], bucket) => {
      const { wrapper, router } = await open()
      vi.mocked(analytics.getOverview).mockClear()
      vi.mocked(analytics.getOverviewSeries).mockClear()

      await button(wrapper, name).trigger('click')
      await settle()

      expect(router.currentRoute.value.query).toEqual(query)
      expect(button(wrapper, name).attributes('aria-pressed')).toBe('true')
      expect(button(wrapper, 'Tümü').attributes('aria-pressed')).toBe('false')
      expect(analytics.getOverview).toHaveBeenCalledWith({ from, to })
      expect(analytics.getOverviewSeries).toHaveBeenCalledWith({ from, to, bucket })
    },
  )

  it('returns to "Tümü" by dropping the query', async () => {
    const { wrapper, router } = await open('/?from=2026-10-24&to=2026-10-30')
    expect(button(wrapper, 'Son 7 gün').attributes('aria-pressed')).toBe('true')

    await button(wrapper, 'Tümü').trigger('click')
    await settle()

    expect(router.currentRoute.value.query).toEqual({})
    expect(analytics.getOverview).toHaveBeenLastCalledWith({})
  })

  it('survives a reload: the range of the URL is applied and a preset is recognised', async () => {
    await open('/?from=2026-10-10&to=2026-10-12')

    expect(analytics.getOverview).toHaveBeenCalledWith({
      from: utc(2026, 10, 10),
      to: utc(2026, 10, 13),
    })
    expect(analytics.getOverviewSeries).toHaveBeenCalledWith({
      from: utc(2026, 10, 10),
      to: utc(2026, 10, 13),
      bucket: 'day',
    })
    // A range that is no preset shows as the custom one, with its days.
    const custom = wrapper!.findAll('main button').find((b) => b.text().includes('–'))!
    expect(custom.text()).toBe('10 Eki 2026 – 12 Eki 2026')
    expect(custom.attributes('aria-pressed')).toBe('true')
  })

  it('ignores a hand-edited range that is not a date', async () => {
    const { wrapper } = await open('/?from=yesterday&to=2026-13-45')
    expect(analytics.getOverview).toHaveBeenCalledWith({})
    expect(button(wrapper, 'Tümü').attributes('aria-pressed')).toBe('true')
  })

  it('takes a custom range from the calendar once both days are picked', async () => {
    const { wrapper, router } = await open()
    await wrapper
      .findAll('main button')
      .find((b) => b.text() === 'Özel aralık')!
      .trigger('click')
    await settle()
    const calendar = wrapper.findComponent(RangeCalendar)
    expect(calendar.exists()).toBe(true)

    // The first click only starts the range.
    calendar.vm.$emit('update:modelValue', { start: new CalendarDate(2026, 10, 5), end: undefined })
    await settle()
    expect(router.currentRoute.value.query).toEqual({})

    vi.mocked(analytics.getOverview).mockClear()
    calendar.vm.$emit('update:modelValue', {
      start: new CalendarDate(2026, 10, 5),
      end: new CalendarDate(2026, 10, 9),
    })
    await settle()

    expect(router.currentRoute.value.query).toEqual({ from: '2026-10-05', to: '2026-10-09' })
    expect(analytics.getOverview).toHaveBeenCalledWith({
      from: utc(2026, 10, 5),
      to: utc(2026, 10, 10),
    })
    expect(wrapper.findAll('main button').some((b) => b.text() === '5 Eki 2026 – 9 Eki 2026')).toBe(
      true,
    )
    // The popover closed.
    expect(wrapper.findComponent(RangeCalendar).exists()).toBe(false)
  })

  it('sends a single-day custom range as one day', async () => {
    const { wrapper } = await open()
    await wrapper
      .findAll('main button')
      .find((b) => b.text() === 'Özel aralık')!
      .trigger('click')
    await settle()
    wrapper.findComponent(RangeCalendar).vm.$emit('update:modelValue', {
      start: new CalendarDate(2026, 10, 12),
      end: new CalendarDate(2026, 10, 12),
    })
    await settle()
    expect(analytics.getOverviewSeries).toHaveBeenLastCalledWith({
      from: utc(2026, 10, 12),
      to: utc(2026, 10, 13),
      bucket: 'hour',
    })
  })
})
