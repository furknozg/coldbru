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

  it('puts the approval of a step before the opening of the next one at the same instant', () => {
    const audience = stepOf('audience', {
      id: 's1',
      created_at: at(1),
      approved_at: at(2),
      status: 'approved',
    })
    audience.variants = [
      variantOf({ id: 'v1', step_id: 's1', created_at: at(1), approved_at: at(2) }),
    ]
    const plan = stepOf('channel_plan', { id: 's2', created_at: at(2) })
    plan.variants = [
      variantOf({ id: 'v2', step_id: 's2', kind: 'channel_plan', created_at: at(2) }),
    ]
    const items = build(viewOf({ created_at: at(0), steps: [audience, plan] }), [
      {
        name: 'screen_result',
        data: { at: at(0), campaign_id: 'c1', verdict: 'allow', reason: '' },
      },
      { name: 'campaign_cancelled', data: { at: at(2), campaign_id: 'c1', reason: 'x' } },
    ])

    expect(items.map((i) => `${i.kind}:${i.stepId ?? '-'}`)).toEqual([
      'brief:-',
      'screen:-',
      'stepOpened:s1',
      'variantCreated:s1',
      'variantApproved:s1',
      'stepApproved:s1',
      'stepOpened:s2',
      'variantCreated:s2',
      'campaignCancelled:-',
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

  it('turns chat messages into cards: questions and summaries as agent markdown, answers as the marketer’s', () => {
    const step = stepOf('content', { id: 's3', created_at: at(1) })
    const items = build(
      viewOf({
        steps: [step],
        messages: [
          {
            id: 'q',
            campaign_id: 'c1',
            step_id: 's3',
            kind: 'question',
            body: 'Hangi ton?',
            options: ['Samimi', 'Resmi'],
            created_at: at(2),
          },
          {
            id: 'a',
            campaign_id: 'c1',
            step_id: 's3',
            kind: 'answer',
            body: 'Samimi',
            reply_to: 'q',
            created_at: at(3),
          },
          {
            id: 'm',
            campaign_id: 'c1',
            step_id: 's3',
            kind: 'summary',
            body: '**kısa**',
            created_at: at(4),
          },
        ],
      }),
      [
        {
          name: 'summary_posted',
          data: {
            at: at(5),
            campaign_id: 'c1',
            step_id: 's3',
            message_id: 'm2',
            slot: { language: 'tr', channel: 'sms' },
            markdown: 'SMS özeti',
          },
        },
      ],
    )

    const byKind = Object.fromEntries(items.map((i) => [i.key, i]))
    expect(byKind['message:q']).toMatchObject({
      kind: 'question',
      author: 'system',
      markdown: true,
      title: 'Metin için soru',
      question: { id: 'q', options: ['Samimi', 'Resmi'] },
      stepId: 's3',
    })
    expect(byKind['message:a']).toMatchObject({
      kind: 'answer',
      author: 'user',
      title: 'Cevap',
      body: 'Samimi',
    })
    expect(byKind['message:a']!.markdown).toBeUndefined()
    expect(byKind['message:m']).toMatchObject({
      kind: 'summary',
      markdown: true,
      title: 'Metin özeti',
    })
    expect(byKind['message:m2']!.title).toBe('Metin · Türkçe · SMS özeti')
    expect(items.map((i) => i.kind)).toEqual([
      'brief',
      'stepOpened',
      'question',
      'answer',
      'summary',
      'summary',
    ])
  })

  it('gives an approved step its approval card even without approved_at', () => {
    const step = stepOf('audience', { id: 's1', created_at: at(2), status: 'approved' })
    const approved = build(viewOf({ steps: [step] })).find((i) => i.kind === 'stepApproved')
    expect(approved).toMatchObject({ at: at(2), stepId: 's1' })
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
