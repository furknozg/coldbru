import { describe, expect, it } from 'vitest'

import type { LlmSeries, LlmSeriesPoint, OverviewSeries } from '@/api/analytics/service'

import {
  channelSlices,
  deliveriesWithStatus,
  modelSlices,
  purposeSlices,
  statusSlices,
} from '../overview'
import { OTHER_KEY, deliveryLines, llmCallsLine, llmCostLines, modelKeys, pivot } from '../series'

const days = ['2026-10-28T00:00:00Z', '2026-10-29T00:00:00Z', '2026-10-30T00:00:00Z']

describe('pivot', () => {
  it('lays sparse rows on every bucket and fills the rest with zero', () => {
    const lines = pivot(
      days,
      [
        { start: '2026-10-29T00:00:00Z', k: 'a', v: 4 },
        { start: '2026-10-29T00:00:00Z', k: 'a', v: 1 },
        { start: '2026-10-30T00:00:00Z', k: 'b', v: 7 },
      ],
      ['a', 'b', 'c'],
      (p) => p.k,
      (p) => p.v,
    )
    expect(lines).toEqual([
      { key: 'a', values: [0, 5, 0] },
      { key: 'b', values: [0, 0, 7] },
      { key: 'c', values: [0, 0, 0] },
    ])
  })

  it('matches buckets by instant and ignores rows outside them or of another key', () => {
    const lines = pivot(
      days,
      [
        { start: '2026-10-29T03:00:00+03:00', k: 'a', v: 2 },
        { start: '2026-11-05T00:00:00Z', k: 'a', v: 9 },
        { start: '2026-10-28T00:00:00Z', k: 'z', v: 9 },
      ],
      ['a'],
      (p) => p.k,
      (p) => p.v,
    )
    // 03:00 at +03:00 is midnight UTC of the 29th.
    expect(lines).toEqual([{ key: 'a', values: [0, 2, 0] }])
  })

  it('gives all-zero lines for a series without points', () => {
    expect(
      pivot(
        days,
        [],
        ['a'],
        () => 'a',
        () => 1,
      ),
    ).toEqual([{ key: 'a', values: [0, 0, 0] }])
    expect(
      pivot(
        [],
        [],
        ['a'],
        () => 'a',
        () => 1,
      ),
    ).toEqual([{ key: 'a', values: [] }])
  })
})

describe('delivery series', () => {
  const series: OverviewSeries = {
    bucket: 'day',
    buckets: days,
    from: days[0]!,
    to: '2026-10-31T00:00:00Z',
    points: [
      { start: days[1]!, channel: 'email', status: 'sent', count: 10 },
      { start: days[1]!, channel: 'email', status: 'failed', count: 3 },
      { start: days[1]!, channel: 'sms', status: 'sent', count: 6 },
      { start: days[2]!, channel: 'email', status: 'sent', count: 5 },
    ],
  }

  it('counts the sent deliveries of each channel per bucket, zero-filled', () => {
    expect(deliveryLines(series)).toEqual([
      { key: 'email', values: [0, 10, 5] },
      { key: 'sms', values: [0, 6, 0] },
    ])
  })

  it('can count another status', () => {
    expect(deliveryLines(series, 'failed')[0]).toEqual({ key: 'email', values: [0, 3, 0] })
  })
})

describe('LLM series', () => {
  const point = (over: Partial<LlmSeriesPoint>): LlmSeriesPoint => ({
    start: days[0]!,
    purpose: 'content',
    model: 'claude-sonnet-5-5',
    calls: 1,
    cost_usd: '0.01000000',
    input_tokens: 100,
    output_tokens: 50,
    ...over,
  })
  const series = (points: LlmSeriesPoint[]): LlmSeries => ({
    bucket: 'day',
    buckets: days,
    from: days[0]!,
    to: '2026-10-31T00:00:00Z',
    points,
  })

  it('stacks cost by purpose in the order of the flow, every purpose present', () => {
    const lines = llmCostLines(
      series([
        point({ purpose: 'content', cost_usd: '0.5' }),
        point({ purpose: 'screen', start: days[1]!, cost_usd: '0.25' }),
        point({ purpose: 'content', start: days[1]!, cost_usd: '0.125' }),
      ]),
      'purpose',
    )
    expect(lines.map((l) => l.key)).toEqual([
      'screen',
      'segment',
      'channel_plan',
      'content',
      'schedule',
    ])
    expect(lines.find((l) => l.key === 'content')!.values).toEqual([0.5, 0.125, 0])
    expect(lines.find((l) => l.key === 'screen')!.values).toEqual([0, 0.25, 0])
    expect(lines.find((l) => l.key === 'schedule')!.values).toEqual([0, 0, 0])
  })

  it('stacks cost by model in name order and sums the calls of all rows', () => {
    const s = series([
      point({ model: 'claude-sonnet-5-5', cost_usd: '1', calls: 2 }),
      point({ model: 'claude-haiku-5-5', cost_usd: '0.5', calls: 3, start: days[2]! }),
    ])
    expect(llmCostLines(s, 'model')).toEqual([
      { key: 'claude-haiku-5-5', values: [0, 0, 0.5] },
      { key: 'claude-sonnet-5-5', values: [1, 0, 0] },
    ])
    expect(llmCallsLine(s)).toEqual({ key: 'calls', values: [2, 0, 3] })
  })

  it('folds the smallest models into "other" beyond five lines', () => {
    const models = ['m1', 'm2', 'm3', 'm4', 'm5', 'm6']
    const s = series(models.map((model, i) => point({ model, cost_usd: String(10 - i) })))
    expect(modelKeys(s)).toEqual(['m1', 'm2', 'm3', 'm4', OTHER_KEY])
    const other = llmCostLines(s, 'model').find((l) => l.key === OTHER_KEY)!
    expect(other.values).toEqual([6 + 5, 0, 0])
  })

  it('treats a cost that is not a number as zero', () => {
    const lines = llmCostLines(series([point({ cost_usd: 'oops' })]), 'purpose')
    expect(lines.find((l) => l.key === 'content')!.values).toEqual([0, 0, 0])
  })
})

describe('distribution slices', () => {
  const counts = [
    { channel: 'email' as const, status: 'sent' as const, count: 80 },
    { channel: 'email' as const, status: 'failed' as const, count: 5 },
    { channel: 'sms' as const, status: 'sent' as const, count: 40 },
    { channel: 'sms' as const, status: 'skipped_no_consent' as const, count: 15 },
  ]

  it('sums deliveries by channel (both listed) and by status (empty ones left out)', () => {
    expect(channelSlices(counts, (c) => c.toUpperCase()).map((s) => [s.key, s.value])).toEqual([
      ['email', 85],
      ['sms', 55],
    ])
    expect(statusSlices(counts, (s) => s).map((s) => [s.key, s.value])).toEqual([
      ['sent', 120],
      ['failed', 5],
      ['skipped_no_consent', 15],
    ])
    expect(deliveriesWithStatus(counts, 'sent')).toBe(120)
    expect(deliveriesWithStatus(counts, 'pending')).toBe(0)
  })

  it('splits LLM cost by purpose and by model, folding the tail of the models', () => {
    const byPurpose = [
      { purpose: 'content' as const, calls: 2, cost_usd: '0.4', input_tokens: 1, output_tokens: 1 },
      { purpose: 'screen' as const, calls: 1, cost_usd: '0.1', input_tokens: 1, output_tokens: 1 },
    ]
    expect(purposeSlices(byPurpose, (p) => p).map((s) => [s.key, s.value])).toEqual([
      ['screen', 0.1],
      ['content', 0.4],
    ])
    const models = ['a', 'b', 'c', 'd', 'e', 'f'].map((model, i) => ({
      model,
      calls: 1,
      cost_usd: String(i + 1),
      input_tokens: 0,
      output_tokens: 0,
    }))
    const slices = modelSlices(models, 'Diğer')
    expect(slices.map((s) => s.key)).toEqual(['f', 'e', 'd', 'c', OTHER_KEY])
    expect(slices[slices.length - 1]!.value).toBe(1 + 2)
  })
})
