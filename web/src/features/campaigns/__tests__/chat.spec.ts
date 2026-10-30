import { describe, expect, it } from 'vitest'

import { createAppI18n } from '@/i18n'
import { infoOf, stepOf, variantOf, viewOf } from '@/__tests__/helpers'

import { buildChat, isMessage } from '../chat'
import { groupByStatus } from '../groups'
import { createLabels } from '../labels'
import { buildTimeline, type LiveEntry } from '../timeline'

const { global: i18n } = createAppI18n()
const labels = createLabels(i18n)
const at = (s: number) => `2026-10-30T10:00:${String(s).padStart(2, '0')}.000Z`

const chat = (view: ReturnType<typeof viewOf>, live: LiveEntry[] = []) =>
  buildChat(view, buildTimeline(view, live, i18n, labels))
const shape = (entries: ReturnType<typeof chat>) =>
  entries.map((e) => (e.type === 'step' ? `step:${e.step.id}` : e.item.kind))

describe('buildChat', () => {
  it('files the events of a step under the step and leaves messages and campaign events as lines', () => {
    const audience = stepOf('audience', {
      id: 's1',
      created_at: at(1),
      approved_at: at(5),
      status: 'approved',
    })
    audience.variants = [
      variantOf({
        id: 'v1',
        step_id: 's1',
        created_at: at(2),
        approved_at: at(5),
        status: 'approved',
      }),
    ]
    const plan = stepOf('channel_plan', { id: 's2', created_at: at(5) })
    const view = viewOf({
      created_at: at(0),
      steps: [audience, plan],
      notes: [{ id: 'n1', body: 'not', created_at: at(3), step_id: 's1' }],
    })
    const entries = chat(view, [
      {
        name: 'screen_result',
        data: { at: at(0), campaign_id: 'c1', verdict: 'allow', reason: '' },
      },
    ])

    // The approved step stands where it was approved, the open one stands last.
    expect(shape(entries)).toEqual(['brief', 'screen', 'note', 'step:s1', 'step:s2'])
    const first = entries.find((e) => e.type === 'step' && e.step.id === 's1')!
    expect(first.type === 'step' && first.events.map((e) => e.kind)).toEqual([
      'stepOpened',
      'variantCreated',
      'variantApproved',
      'stepApproved',
    ])
  })

  it('keeps an event of a step the view does not know as a line', () => {
    const view = viewOf({ created_at: at(0) })
    const entries = chat(view, [
      {
        name: 'generation_failed',
        data: {
          at: at(3),
          campaign_id: 'c1',
          step_id: 'ahead',
          kind: 'content',
          slot: {},
          reason: 'x',
        },
      },
    ])

    expect(shape(entries)).toEqual(['brief', 'generationFailed'])
  })

  it('puts the open step after the campaign events, even later ones', () => {
    const open = stepOf('audience', { id: 's1', created_at: at(1) })
    const view = viewOf({ status: 'cancelled', created_at: at(0), steps: [open] })
    const entries = chat(view, [
      { name: 'campaign_cancelled', data: { at: at(9), campaign_id: 'c1', reason: '' } },
    ])

    expect(shape(entries)).toEqual(['brief', 'campaignCancelled', 'step:s1'])
  })

  it('tells messages from system lines', () => {
    const view = viewOf({ created_at: at(0) })
    const lines = chat(view, [
      { name: 'campaign_cancelled', data: { at: at(2), campaign_id: 'c1', reason: '' } },
    ]).flatMap((e) => (e.type === 'line' ? [e.item] : []))

    expect(lines.map((i) => [i.kind, isMessage(i)])).toEqual([
      ['brief', true],
      ['campaignCancelled', false],
    ])
  })
})

describe('groupByStatus', () => {
  it('orders the groups draft, scheduled, sending, completed, cancelled and drops empty ones', () => {
    const items = [
      infoOf({ id: 'a', status: 'cancelled' }),
      infoOf({ id: 'b', status: 'drafting' }),
      infoOf({ id: 'c', status: 'completed' }),
      infoOf({ id: 'd', status: 'drafting' }),
    ]

    const groups = groupByStatus(items)

    expect(groups.map((g) => g.status)).toEqual(['drafting', 'completed', 'cancelled'])
    // The order of the list inside a group is kept.
    expect(groups[0]!.items.map((c) => c.id)).toEqual(['b', 'd'])
    expect(groupByStatus([])).toEqual([])
  })
})
