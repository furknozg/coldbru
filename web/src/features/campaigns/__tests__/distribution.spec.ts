import { describe, expect, it } from 'vitest'

import { audienceContent } from '../content'
import { bySize, foldSmallest, languageTotals, NEUTRAL_COLOR } from '../distribution'
import { audienceContent as sample, variantOf } from '@/__tests__/helpers'

describe('languageTotals', () => {
  it('sums by_locale rows per language', () => {
    const content = {
      ...sample,
      by_locale: [
        { language: 'tr', timezone: 'Europe/Istanbul', count: 30, coverage: sample.coverage },
        { language: 'tr', timezone: 'Europe/Berlin', count: 12, coverage: sample.coverage },
        { language: 'en', timezone: 'Europe/London', count: 6, coverage: sample.coverage },
      ],
    }
    const totals = languageTotals(audienceContent(variantOf({ content })))
    expect(totals).toEqual([
      { language: 'tr', count: 42 },
      { language: 'en', count: 6 },
    ])
  })

  it('falls back to by_language when the variant has no by_locale', () => {
    const content = { ...sample, by_locale: undefined }
    const totals = languageTotals(audienceContent(variantOf({ content })))
    expect(totals).toEqual([
      { language: 'tr', count: 42 },
      { language: 'en', count: 6 },
    ])
  })
})

describe('foldSmallest', () => {
  const other = { key: 'other', label: 'Diğer', value: 0 }
  const slices = ['a', 'b', 'c', 'd'].map((key, i) => ({ key, label: key, value: 10 - i * 2 }))

  it('leaves a short list alone', () => {
    expect(foldSmallest(slices, 4, other)).toBe(slices)
  })

  it('keeps the largest and sums the rest into a neutral "other"', () => {
    const folded = foldSmallest(slices, 2, other)
    expect(folded.map((s) => [s.key, s.value])).toEqual([
      ['a', 10],
      ['b', 8],
      ['other', 10],
    ])
    expect(folded[2]!.color).toBe(NEUTRAL_COLOR)
  })
})

describe('bySize', () => {
  it('orders by value, then key, without changing the input', () => {
    const input = [
      { key: 'b', value: 1 },
      { key: 'a', value: 1 },
      { key: 'c', value: 5 },
    ]
    expect(bySize(input).map((s) => s.key)).toEqual(['c', 'a', 'b'])
    expect(input.map((s) => s.key)).toEqual(['b', 'a', 'c'])
  })
})
