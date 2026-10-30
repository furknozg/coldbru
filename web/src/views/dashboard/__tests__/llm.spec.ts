import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import * as analytics from '@/api/analytics/service'
import { FakeEventSource, mountApp } from '@/__tests__/helpers'
import { resetToasts } from '@/features/campaigns/composables/useToasts'
import { llmSeriesOf, overviewOf } from '@/features/dashboard/__tests__/fixtures'

import { freezeToday, legend, mockDashboardApi, settle, utc } from './harness'

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

async function open(path = '/analytics/llm') {
  const mounted = await mountApp(path)
  await settle()
  wrapper = mounted.wrapper
  return mounted
}

const chart = (w: NonNullable<typeof wrapper>, title: string) =>
  w.findAll('figure').find((f) => f.find('figcaption').text() === title)!
const toggle = (w: NonNullable<typeof wrapper>, text: string) =>
  w.findAll('main button').find((b) => b.text() === text)!

describe('LLM analitiği', () => {
  it('shows calls, cost and input and output tokens of the range', async () => {
    const { wrapper } = await open()
    const stat = (key: string) => wrapper.get(`[data-stat="${key}"]`).text()

    expect(stat('calls')).toContain('56')
    expect(stat('cost')).toContain('$0,012345')
    expect(stat('input')).toContain('100.000')
    expect(stat('output')).toContain('20.000')
    // The overview's `llm.tokens` carries the cache tokens too.
    expect(stat('cache')).toContain('Önbellek tokenı')
    expect(stat('cache')).toContain('350.000')
    expect(stat('cache')).toContain('okuma 300.000 · yazma 50.000')
  })

  it('follows the date filter in the URL for the figures, the table and the series', async () => {
    const { wrapper, router } = await open('/analytics/llm?from=2026-10-24&to=2026-10-30')
    const range = { from: utc(2026, 10, 24), to: utc(2026, 10, 31) }

    expect(analytics.getOverview).toHaveBeenCalledWith(range)
    expect(analytics.getPromptVersionUsage).toHaveBeenCalledWith(range)
    expect(analytics.getLlmSeries).toHaveBeenCalledWith({ ...range, bucket: 'day' })

    await wrapper
      .findAll('main button')
      .find((b) => b.text() === 'Bugün')!
      .trigger('click')
    await settle()
    expect(router.currentRoute.value.query).toEqual({ from: '2026-10-30', to: '2026-10-30' })
    expect(analytics.getPromptVersionUsage).toHaveBeenLastCalledWith({
      from: utc(2026, 10, 30),
      to: utc(2026, 10, 31),
    })
    expect(analytics.getLlmSeries).toHaveBeenLastCalledWith({
      from: utc(2026, 10, 30),
      to: utc(2026, 10, 31),
      bucket: 'hour',
    })
  })

  it('splits the cost by purpose and by model in pies with dollar amounts', async () => {
    const { wrapper } = await open()

    expect(legend(wrapper, 'Amaca göre maliyet').map((row) => row.slice(0, 2))).toEqual([
      ['Ön eleme', '$0,001'],
      ['Metin', '$0,01'],
    ])
    expect(legend(wrapper, 'Modele göre maliyet').map((row) => row.slice(0, 2))).toEqual([
      ['claude-haiku-5-5', '$0,001'],
      ['claude-sonnet-5-5', '$0,011345'],
    ])
  })

  it('lists every prompt version with its outcomes, repair rate, latency and cost', async () => {
    const { wrapper } = await open()

    const rows = wrapper.findAll('[data-row="prompt-version"]')
    expect(rows.map((row) => row.findAll('th, td').map((cell) => cell.text()))).toEqual([
      ['content 1.2.0', '20', '17', '1', '1', '1', '%5', '800 ms', '$0,0123'],
      ['screen 1.0.0', '10', '10', '0', '0', '0', '%0', '300 ms', '$0,001'],
    ])
    expect(wrapper.findAll('thead th').map((th) => th.text())).toEqual([
      'Prompt',
      'Çağrı',
      'Kabul',
      'Onarım',
      'Ret',
      'API hatası',
      'Onarım oranı',
      'Ort. gecikme',
      'Maliyet',
    ])
  })

  it('draws the versions as columns of the repair rate and switches to the latency', async () => {
    const { wrapper } = await open()

    const bars = chart(wrapper, 'Prompt sürümü başına')
    expect(bars.get('[data-testid="xy-container"]').attributes('data-rows')).toBe('2')
    expect(bars.get('[role="img"]').attributes('aria-label')).toContain(
      'content 1.2.0: %5, screen 1.0.0: %0',
    )
    expect(toggle(wrapper, 'Onarım oranı').attributes('aria-pressed')).toBe('true')

    await toggle(wrapper, 'Ort. gecikme').trigger('click')
    await settle()
    expect(
      chart(wrapper, 'Prompt sürümü başına').get('[role="img"]').attributes('aria-label'),
    ).toContain('content 1.2.0: 800 ms, screen 1.0.0: 300 ms')
  })

  it('has no chart of the versions, and says so, when the range holds no call', async () => {
    vi.mocked(analytics.getPromptVersionUsage).mockResolvedValue([])
    const { wrapper } = await open()
    expect(wrapper.text()).toContain('Bu aralıkta LLM çağrısı yok.')
    expect(wrapper.find('table').exists()).toBe(false)
  })

  it('stacks the cost over time by purpose, switches to the model, and draws the calls as a line', async () => {
    const { wrapper } = await open()

    const area = chart(wrapper, 'LLM maliyeti')
    expect(area.get('[data-testid="area"]').attributes('data-series')).toBe('5')
    expect(area.findAll('li').map((li) => li.text())).toEqual([
      'Ön eleme',
      'Kitle',
      'Kanal planı',
      'Metin',
      'Zamanlama',
    ])
    // $0,001 screen + $0,009 content, zero-filled over three days.
    expect(area.get('[role="img"]').attributes('aria-label')).toContain('Ön eleme: $0,001')
    expect(area.get('[role="img"]').attributes('aria-label')).toContain('Metin: $0,009')

    await toggle(wrapper, 'Modele göre').trigger('click')
    await settle()
    const byModel = chart(wrapper, 'LLM maliyeti')
    expect(byModel.get('[data-testid="area"]').attributes('data-series')).toBe('2')
    expect(byModel.findAll('li').map((li) => li.text())).toEqual([
      'claude-haiku-5-5',
      'claude-sonnet-5-5',
    ])

    const calls = chart(wrapper, 'LLM çağrıları')
    expect(calls.get('[data-testid="line"]').attributes('data-series')).toBe('1')
    expect(calls.get('[data-testid="xy-container"]').attributes('data-rows')).toBe('3')
    expect(calls.get('[role="img"]').attributes('aria-label')).toContain('LLM çağrısı: 11')
    // One series needs no legend box.
    expect(calls.find('li').exists()).toBe(false)
  })

  it('names the 90-day window when the range has no start', async () => {
    const { wrapper } = await open()
    expect(wrapper.text()).toContain('Zaman içinde · Son 90 gün')
  })

  it('draws a chart without any value as one line of text', async () => {
    vi.mocked(analytics.getLlmSeries).mockResolvedValue({ ...llmSeriesOf(), points: [] })
    vi.mocked(analytics.getOverview).mockResolvedValue(
      overviewOf({
        llm: {
          calls: 0,
          cost_usd: '0',
          tokens: { input: 0, output: 0, cache_read: 0, cache_creation: 0 },
          by_purpose: [],
          by_model: [],
        },
      }),
    )
    const { wrapper } = await open()
    expect(chart(wrapper, 'LLM maliyeti').text()).toContain('Gösterilecek veri yok')
    expect(chart(wrapper, 'LLM maliyeti').find('[data-testid="area"]').exists()).toBe(false)
    expect(chart(wrapper, 'Amaca göre maliyet').text()).toContain('Gösterilecek veri yok')
  })
})
