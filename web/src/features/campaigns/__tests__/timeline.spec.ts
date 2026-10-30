import { describe, expect, it } from 'vitest'

import { createAppI18n } from '@/i18n'
import { stepOf, variantOf, viewOf } from '@/__tests__/helpers'

import { createLabels } from '../labels'
import { buildTimeline, type LiveEntry } from '../timeline'

const { global: i18n } = createAppI18n()
const labels = createLabels(i18n)
const build = (view: Parameters<typeof buildTimeline>[0], live: LiveEntry[] = []) =>
  buildTimeline(view, live, i18n, labels)

const at = (s: number) => `2026-10-30T10:00:${String(s).padStart(2, '0')}.000Z`

describe('buildTimeline', () => {
  it('is empty without a view and events', () => {
    expect(build(null)).toEqual([])
  })

  it('puts the brief first and orders the rest by time, then by the order things happen in', () => {
    const step = stepOf('audience', {
      id: 's1',
      created_at: at(1),
      approved_at: at(1),
      status: 'approved',
    })
    step.variants = [variantOf({ id: 'v1', step_id: 's1', created_at: at(1), approved_at: at(1) })]
    const items = build(viewOf({ created_at: at(0), steps: [step] }))

    // All four step cards share one instant: opened, variant ready, variant approved, step approved.
    expect(items.map((i) => i.kind)).toEqual([
      'brief',
      'stepOpened',
      'variantCreated',
      'variantApproved',
      'stepApproved',
    ])
  })

  it('names a step version above 1 and finds the slot of an approved variant in the view', () => {
    const step = stepOf('content', { id: 's2', version: 2, created_at: at(1) })
    step.variants = [
      variantOf({
        id: 'v2',
        step_id: 's2',
        kind: 'content',
        language: 'en',
        channel: 'email',
        created_at: at(2),
      }),
    ]
    const items = build(viewOf({ steps: [step] }), [
      {
        name: 'variant_approved',
        data: { at: at(3), campaign_id: 'c1', step_id: 's2', variant_id: 'v2' },
      },
      {
        name: 'step_superseded',
        data: { at: at(4), campaign_id: 'c1', step_id: 'old', kind: 'channel_plan', version: 3 },
      },
    ])

    const titles = items.map((i) => i.title)
    expect(titles).toContain('Metin (sürüm 2) adımı açıldı')
    expect(titles).toContain('Metin · İngilizce · E-posta varyantı onaylandı')
    expect(titles).toContain('Kanal planı (sürüm 3) adımı geçersiz kılındı')
  })

  it('lets the stored fact win over the live event about the same fact', () => {
    const step = stepOf('audience', { id: 's1', version: 1, created_at: at(5) })
    const items = build(viewOf({ steps: [step] }), [
      {
        name: 'step_opened',
        data: { at: at(2), campaign_id: 'c1', step_id: 's1', kind: 'audience', version: 1 },
      },
    ])

    const opened = items.filter((i) => i.kind === 'stepOpened')
    expect(opened).toHaveLength(1)
    expect(opened[0]!.at).toBe(at(5))
  })

  it('carries the step of a card so a click can focus it, and only user cards are messages', () => {
    const step = stepOf('audience', { id: 's1' })
    const items = build(
      viewOf({ steps: [step], notes: [{ id: 'n', body: 'Not', created_at: at(3) }] }),
    )

    expect(items.find((i) => i.kind === 'stepOpened')!.stepId).toBe('s1')
    expect(items.filter((i) => i.author === 'user').map((i) => i.kind)).toEqual(['brief', 'note'])
  })
})
