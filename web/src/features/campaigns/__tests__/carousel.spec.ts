import { describe, expect, it } from 'vitest'

import { stepOf, variantOf } from '@/__tests__/helpers'

import { carouselGroups, nextPendingIndex, orderSlides, pendingSlotCount } from '../carousel'
import type { VariantView } from '../types'

const text = (
  id: string,
  language: string,
  channel: 'sms' | 'email',
  status: VariantView['status'] = 'draft',
) => variantOf({ id, kind: 'content', language, channel, status })

describe('pendingSlotCount', () => {
  it('counts a slot once, however many drafts it has', () => {
    const step = stepOf('content', {
      variants: [text('a', 'tr', 'sms'), text('b', 'tr', 'sms'), text('c', 'tr', 'email')],
    })
    expect(pendingSlotCount(step)).toBe(2)
  })

  it('does not count a slot with an approved variant, even when it has other drafts', () => {
    const step = stepOf('content', {
      variants: [
        text('a', 'tr', 'sms', 'approved'),
        text('b', 'tr', 'sms'),
        text('c', 'tr', 'email'),
      ],
    })
    expect(pendingSlotCount(step)).toBe(1)
  })

  it('counts a slot that only has rejected tries and a slot nothing was produced for yet', () => {
    const step = stepOf('content', {
      variants: [text('a', 'tr', 'sms', 'rejected'), text('b', 'tr', 'email', 'approved')],
      missing: [{ language: 'en', channel: 'email' }],
    })
    expect(pendingSlotCount(step)).toBe(2)
  })

  it('counts only the slots of the given channel', () => {
    const step = stepOf('content', {
      variants: [
        text('a', 'tr', 'sms'),
        text('b', 'tr', 'email'),
        text('c', 'en', 'email', 'approved'),
        text('d', 'de', 'email', 'rejected'),
      ],
      missing: [{ language: 'en', channel: 'sms' }],
    })
    expect(pendingSlotCount(step, 'email')).toBe(2)
    expect(pendingSlotCount(step, 'sms')).toBe(2)
    expect(pendingSlotCount(step)).toBe(4)
  })

  it('is 0 when every slot is approved', () => {
    const step = stepOf('content', {
      variants: [text('a', 'tr', 'sms', 'approved'), text('b', 'en', 'email', 'approved')],
    })
    expect(pendingSlotCount(step)).toBe(0)
  })
})

describe('nextPendingIndex', () => {
  const slides = [
    text('a', 'tr', 'sms', 'approved'),
    text('b', 'tr', 'email'),
    text('c', 'en', 'email'),
  ]

  it('takes the next draft after the given slide', () => {
    expect(nextPendingIndex(slides, 0)).toBe(1)
    expect(nextPendingIndex(slides, 1)).toBe(2)
  })

  it('wraps around to the first draft', () => {
    expect(nextPendingIndex(slides, 2)).toBe(1)
  })

  it('skips drafts whose slot is approved already', () => {
    const list = [
      text('a', 'tr', 'sms', 'approved'),
      text('b', 'tr', 'sms'),
      text('c', 'en', 'sms'),
    ]
    expect(nextPendingIndex(list, 0)).toBe(2)
  })

  it('is -1 when no slot waits any more', () => {
    const done = [text('a', 'tr', 'sms', 'approved'), text('b', 'tr', 'email', 'approved')]
    expect(nextPendingIndex(done, 1)).toBe(-1)
    expect(nextPendingIndex([], 0)).toBe(-1)
  })
})

describe('orderSlides', () => {
  const ids = (list: VariantView[]) => list.map((v) => v.id)

  it('orders by language, then time zone, and keeps the listed order inside one slot', () => {
    const zone = (id: string, language: string, timezone: string) =>
      variantOf({ id, kind: 'schedule', language, timezone })
    const list = [
      zone('tr-ist', 'tr', 'Europe/Istanbul'),
      zone('en', 'en', 'Europe/London'),
      zone('tr-ber', 'tr', 'Europe/Berlin'),
      zone('tr-ist-2', 'tr', 'Europe/Istanbul'),
    ]
    expect(ids(orderSlides(list))).toEqual(['en', 'tr-ber', 'tr-ist', 'tr-ist-2'])
  })

  it('does not change its input', () => {
    const list = [text('b', 'tr', 'sms'), text('a', 'en', 'sms')]
    orderSlides(list)
    expect(ids(list)).toEqual(['b', 'a'])
  })
})

describe('carouselGroups', () => {
  it('gives the text step one group per channel, e-mail first, and skips an empty channel', () => {
    const variants = [text('s', 'tr', 'sms'), text('m', 'tr', 'email'), text('m2', 'en', 'email')]
    const step = stepOf('content', { variants })
    const groups = carouselGroups(step, variants, true)
    expect(groups.map((g) => [g.key, g.channel, ids(g.variants), g.pending])).toEqual([
      ['email', 'email', ['m', 'm2'], 2],
      ['sms', 'sms', ['s'], 1],
    ])
    const mails = variants.filter((v) => v.channel === 'email')
    // Without SMS cards the SMS slot still waits: its group stays, with no slide.
    expect(carouselGroups(step, mails, true).map((g) => [g.key, g.variants.length])).toEqual([
      ['email', 2],
      ['sms', 0],
    ])
  })

  it('keeps a group that has no card but slots waiting, and drops one without any slot', () => {
    const variants = [text('m', 'tr', 'email'), text('r', 'tr', 'sms', 'rejected')]
    const step = stepOf('content', { variants, missing: [{ language: 'en', channel: 'sms' }] })
    const groups = carouselGroups(step, [variants[0]!], true)
    expect(groups.map((g) => [g.key, ids(g.variants), g.pending])).toEqual([
      ['email', ['m'], 1],
      ['sms', [], 2],
    ])
    const emailOnly = stepOf('content', { variants: [variants[0]!] })
    expect(carouselGroups(emailOnly, [variants[0]!], true).map((g) => g.key)).toEqual(['email'])
    // Approved: nothing waits, so a group without a card is gone.
    expect(carouselGroups(step, [variants[0]!], false).map((g) => g.key)).toEqual(['email'])
  })

  it('has no pending count once the step is not open', () => {
    const variants = [text('s', 'tr', 'sms', 'approved')]
    const groups = carouselGroups(stepOf('content', { variants }), variants, false)
    expect(groups.map((g) => g.pending)).toEqual([0])
  })

  it('gives the schedule step one group for all its slots', () => {
    const variants = [
      variantOf({ id: 'a', kind: 'schedule', language: 'tr', timezone: 'Europe/Istanbul' }),
      variantOf({ id: 'b', kind: 'schedule', language: 'en', timezone: 'Europe/London' }),
    ]
    const step = stepOf('schedule', { variants })
    const groups = carouselGroups(step, variants, true)
    expect(groups.map((g) => [g.key, g.channel, g.pending])).toEqual([['schedule', undefined, 2]])
    expect(carouselGroups(step, [], true).map((g) => [g.variants.length, g.pending])).toEqual([
      [0, 2],
    ])
    expect(carouselGroups(stepOf('schedule'), [], true)).toEqual([])
  })
})

function ids(list: VariantView[]) {
  return list.map((v) => v.id)
}
