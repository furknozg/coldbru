import { DOMWrapper, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import * as analytics from '@/api/analytics/service'
import * as proposal from '@/api/proposal/service'
import { resetToasts } from '@/features/campaigns/composables/useToasts'
import type { CampaignView, StepView, VariantView } from '@/features/campaigns/types'
import { FakeEventSource, mountApp, stepOf, variantOf, viewOf } from '@/__tests__/helpers'

vi.mock('@/api/proposal/service')
vi.mock('@/api/analytics/service')

let wrapper: Awaited<ReturnType<typeof mountApp>>['wrapper'] | undefined

beforeEach(() => {
  FakeEventSource.install()
  vi.mocked(proposal.listCampaigns).mockResolvedValue([])
  vi.mocked(analytics.getCampaignSummary).mockResolvedValue(null)
  vi.mocked(proposal.previewVariant).mockResolvedValue({
    subject: 'Your cart Alex',
    html: '<!DOCTYPE html><html><body><p>x</p></body></html>',
    text: 'x',
  })
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  document.body.innerHTML = ''
  resetToasts()
  vi.resetAllMocks()
})

async function open(view: CampaignView) {
  vi.mocked(proposal.getCampaign).mockResolvedValue(view)
  const mounted = await mountApp(`/campaigns/${view.id}`)
  wrapper = mounted.wrapper
  await vi.dynamicImportSettled()
  await flushPromises()
}

const emailStored = {
  subject: 'Your cart',
  preheader: 'Pre',
  headline: 'Head',
  paragraphs: ['One', 'Two'],
  cta_label: 'Go',
}

const text = (
  id: string,
  language: string,
  channel: 'sms' | 'email',
  status: VariantView['status'] = 'draft',
) =>
  variantOf({
    id,
    step_id: 'step-content',
    kind: 'content',
    language,
    channel,
    status,
    content:
      channel === 'sms'
        ? { text: `SMS ${id}`, encoding: 'gsm7', segments: 1, rendered_length: 8 }
        : emailStored,
  })

/** Audience and plan approved; the content step has these variants and is `status`. */
function steps(variants: VariantView[], status: StepView['status'] = 'open'): StepView[] {
  const approved = { status: 'approved', approved_at: '2026-10-30T10:00:01Z' } as const
  return [
    stepOf('audience', { id: 'step-audience', ...approved }),
    stepOf('channel_plan', { id: 'step-channel_plan', ...approved }),
    stepOf('content', { id: 'step-content', status, variants }),
  ]
}

/** Audience, plan and text approved; the schedule step has these variants and is `status`. */
function scheduleSteps(variants: VariantView[], status: StepView['status'] = 'open'): StepView[] {
  const approved = { status: 'approved', approved_at: '2026-10-30T10:00:01Z' } as const
  return [
    stepOf('audience', { id: 'step-audience', ...approved }),
    stepOf('channel_plan', { id: 'step-channel_plan', ...approved }),
    stepOf('content', { id: 'step-content', ...approved }),
    stepOf('schedule', { id: 'step-schedule', status, variants }),
  ]
}

const slot = (
  id: string,
  language: string,
  timezone: string,
  status: VariantView['status'] = 'draft',
) =>
  variantOf({
    id,
    step_id: 'step-schedule',
    kind: 'schedule',
    language,
    timezone,
    status,
    content: { send_at: '2026-11-01T06:30:00Z', rationale: `Neden ${id}` },
  })

/** The carousel of a channel (or of the schedule step), and what is in it. */
const carousel = (key: string) =>
  document.body.querySelector<HTMLElement>(`[data-slot="carousel"][data-carousel="${key}"]`)
const carousels = () => [...document.body.querySelectorAll<HTMLElement>('[data-slot="carousel"]')]
const slidesOf = (key: string) => [
  ...carousel(key)!.querySelectorAll<HTMLElement>('[data-slot="carousel-item"]'),
]
const positionOf = (key: string) =>
  carousel(key)?.querySelector('[data-position]')?.textContent?.trim()
const badgeOf = (key: string) => carousel(key)?.querySelector<HTMLElement>('[data-pending]') ?? null
const countOf = (key: string) => badgeOf(key)?.textContent?.trim()
const titlesOf = (key: string) =>
  slidesOf(key).map((s) => s.querySelector('article')!.getAttribute('aria-label'))
/** The slides the reader can reach in a carousel: the others are inert. */
const shownIn = (key: string) => slidesOf(key).filter((s) => !s.hasAttribute('inert'))

function button(label: string, root: ParentNode = document.body) {
  const found = [...root.querySelectorAll('button')].find((b) => b.textContent?.trim() === label)
  if (!found) throw new Error(`no button "${label}"`)
  return new DOMWrapper(found)
}

async function next(key: string) {
  await button('Sonraki varyant', carousel(key)!).trigger('click')
  await flushPromises()
}

async function approve(key: string, index: number, answer: VariantView[], flow = steps) {
  vi.mocked(proposal.approveVariant).mockResolvedValueOnce(viewOf({ steps: flow(answer) }))
  await button('Onayla', slidesOf(key)[index]!).trigger('click')
  await flushPromises()
}

const approved = (v: VariantView): VariantView => ({ ...v, status: 'approved' })

/** Two e-mails and two SMS, each in Turkish and English; the step lists them mixed. */
const fourSlots = () => [
  text('sms-tr', 'tr', 'sms'),
  text('mail-tr', 'tr', 'email'),
  text('sms-en', 'en', 'sms'),
  text('mail-en', 'en', 'email'),
]

describe('text step carousels, one per channel (D-095)', () => {
  it('stacks an e-mail carousel and an SMS carousel, each with its label, position and own order', async () => {
    await open(viewOf({ steps: steps(fourSlots()) }))

    expect(carousels().map((c) => c.getAttribute('data-carousel'))).toEqual(['email', 'sms'])
    expect(carousel('email')!.getAttribute('aria-label')).toBe('Varyantlar: E-posta')
    expect(carousel('sms')!.getAttribute('aria-label')).toBe('Varyantlar: SMS')
    expect(
      carousels().map((c) => c.querySelector('[data-carousel-label]')!.textContent?.trim()),
    ).toEqual(['E-posta', 'SMS'])
    // Inside a carousel: language order, whatever order the step lists the variants in.
    expect(titlesOf('email')).toEqual(['İngilizce · E-posta', 'Türkçe · E-posta'])
    expect(titlesOf('sms')).toEqual(['İngilizce · SMS', 'Türkçe · SMS'])
    expect(positionOf('email')).toBe('1 / 2')
    expect(positionOf('sms')).toBe('1 / 2')
    expect(shownIn('email')).toHaveLength(1)
    expect(shownIn('sms')).toHaveLength(1)
  })

  it('renders a carousel only for a channel that has slides', async () => {
    await open(
      viewOf({ steps: steps([text('mail-tr', 'tr', 'email'), text('mail-en', 'en', 'email')]) }),
    )
    expect(carousels().map((c) => c.getAttribute('data-carousel'))).toEqual(['email'])
    expect(carousel('sms')).toBeNull()
  })

  it('moves one carousel with its arrows and leaves the other alone', async () => {
    await open(viewOf({ steps: steps(fourSlots()) }))
    const prev = () => button('Önceki varyant', carousel('email')!).element as HTMLButtonElement
    expect(prev().disabled).toBe(true)

    await next('email')
    expect(positionOf('email')).toBe('2 / 2')
    expect(positionOf('sms')).toBe('1 / 2')
    expect(shownIn('email')[0]!.querySelector('article')!.getAttribute('aria-label')).toBe(
      'Türkçe · E-posta',
    )
    expect(
      (button('Sonraki varyant', carousel('email')!).element as HTMLButtonElement).disabled,
    ).toBe(true)
    // At rest only the shown slide takes up height.
    expect(slidesOf('email').map((s) => s.classList.contains('h-0'))).toEqual([true, false])

    await button('Önceki varyant', carousel('email')!).trigger('click')
    await flushPromises()
    expect(positionOf('email')).toBe('1 / 2')
  })

  it('moves with the arrow keys of the focused carousel, but leaves a text box alone', async () => {
    await open(viewOf({ steps: steps(fourSlots()) }))
    await new DOMWrapper(carousel('sms')!).trigger('keydown', { key: 'ArrowRight' })
    await flushPromises()
    expect(positionOf('sms')).toBe('2 / 2')
    expect(positionOf('email')).toBe('1 / 2')

    // The e-mail's second slide: edit it, and press an arrow in the subject field.
    await next('email')
    await button('Düzenle', slidesOf('email')[1]!).trigger('click')
    await flushPromises()
    const subject = slidesOf('email')[1]!.querySelector<HTMLInputElement>('[name="subject"]')!
    await new DOMWrapper(subject).trigger('keydown', { key: 'ArrowLeft' })
    await flushPromises()
    expect(positionOf('email')).toBe('2 / 2')
  })

  it('shows no arrows for a single variant, but the label and the badge', async () => {
    await open(viewOf({ steps: steps([text('sms-tr', 'tr', 'sms')]) }))
    expect(slidesOf('sms')).toHaveLength(1)
    expect(positionOf('sms')).toBeUndefined()
    expect(carousel('sms')!.querySelector('[data-carousel-label]')).not.toBeNull()
    expect(countOf('sms')).toBe('1')
  })

  it('counts the unapproved slots of each channel in its own round badge', async () => {
    await open(viewOf({ steps: steps(fourSlots()) }))
    for (const key of ['email', 'sms']) {
      expect(countOf(key)).toBe('2')
      expect(badgeOf(key)!.getAttribute('data-slot')).toBe('badge')
      expect(badgeOf(key)!.className).toContain('rounded-full')
      // Over the card area of its own carousel, not in the header of arrows.
      expect(slidesOf(key)[0]!.closest('.relative')!.contains(badgeOf(key))).toBe(true)
    }
    expect(badgeOf('email')!.getAttribute('aria-label')).toBe('E-posta: 2 onay kaldı')
    expect(badgeOf('sms')!.getAttribute('aria-label')).toBe('SMS: 2 onay kaldı')
  })

  it('counts a slot once however many drafts it has, per channel, and hides a badge at 0', async () => {
    const list = [
      text('sms-1', 'tr', 'sms'),
      text('sms-2', 'tr', 'sms'),
      text('mail-tr', 'tr', 'email', 'approved'),
      text('mail-en', 'en', 'email', 'approved'),
    ]
    await open(viewOf({ steps: steps(list) }))
    expect(slidesOf('sms')).toHaveLength(2)
    expect(countOf('sms')).toBe('1')
    expect(badgeOf('email')).toBeNull()
  })

  it('moves on to the next waiting slide of the same channel after an approval, not to the other channel', async () => {
    await open(viewOf({ steps: steps(fourSlots()) }))
    const [smsTr, mailTr, smsEn, mailEn] = fourSlots() as [
      VariantView,
      VariantView,
      VariantView,
      VariantView,
    ]
    await approve('email', 0, [smsTr, mailTr, smsEn, approved(mailEn)])

    expect(proposal.approveVariant).toHaveBeenCalledWith('mail-en')
    expect(positionOf('email')).toBe('2 / 2')
    expect(countOf('email')).toBe('1')
    expect(positionOf('sms')).toBe('1 / 2')
    expect(countOf('sms')).toBe('2')
  })

  it('wraps around within the channel and stays when no slot of it waits', async () => {
    await open(viewOf({ steps: steps(fourSlots()) }))
    const [smsTr, mailTr, smsEn, mailEn] = fourSlots() as [
      VariantView,
      VariantView,
      VariantView,
      VariantView,
    ]
    await next('sms')
    expect(positionOf('sms')).toBe('2 / 2')

    await approve('sms', 1, [approved(smsTr), mailTr, smsEn, mailEn])
    expect(positionOf('sms')).toBe('1 / 2')
    expect(countOf('sms')).toBe('1')
    expect(positionOf('email')).toBe('1 / 2')

    // The last waiting SMS: nothing waits any more, the carousel stays and its badge goes.
    await approve('sms', 0, [approved(smsTr), mailTr, approved(smsEn), mailEn])
    expect(positionOf('sms')).toBe('1 / 2')
    expect(badgeOf('sms')).toBeNull()
    // The e-mails still wait.
    expect(countOf('email')).toBe('2')
    expect(positionOf('email')).toBe('1 / 2')
  })

  it('skips a draft whose slot already has an approved variant', async () => {
    const list = [
      text('sms-1', 'tr', 'sms'),
      text('sms-2', 'tr', 'sms'),
      text('sms-en', 'en', 'sms'),
    ]
    await open(viewOf({ steps: steps(list) }))
    // Slides: English SMS, SMS 1, SMS 2.
    expect(titlesOf('sms')).toEqual(['İngilizce · SMS', 'Türkçe · SMS', 'Türkçe · SMS'])
    await next('sms')
    await approve('sms', 1, [approved(list[0]!), list[1]!, list[2]!])
    expect(proposal.approveVariant).toHaveBeenCalledWith('sms-1')
    // SMS 2 shares the approved slot: skipped; the English SMS is next, wrapping around.
    expect(positionOf('sms')).toBe('1 / 3')
    expect(countOf('sms')).toBe('1')
  })

  it('shows the approved variants of an approved step in the carousels, without badges', async () => {
    const list = [
      text('sms-tr', 'tr', 'sms', 'approved'),
      text('sms-old', 'tr', 'sms', 'rejected'),
      text('mail-en', 'en', 'email', 'approved'),
      text('mail-tr', 'tr', 'email', 'approved'),
    ]
    await open(viewOf({ status: 'scheduled', steps: steps(list, 'approved') }))

    expect(carousels().map((c) => c.getAttribute('data-carousel'))).toEqual(['email', 'sms'])
    expect(titlesOf('email')).toEqual(['İngilizce · E-posta', 'Türkçe · E-posta'])
    expect(titlesOf('sms')).toEqual(['Türkçe · SMS'])
    expect(positionOf('email')).toBe('1 / 2')
    expect(document.body.querySelector('[data-pending]')).toBeNull()
    for (const slide of carousels().flatMap((c) => [
      ...c.querySelectorAll('[data-slot="carousel-item"]'),
    ])) {
      expect(slide.querySelector('article')!.getAttribute('data-variant-status')).toBe('approved')
      expect(slide.querySelector('footer')).toBeNull()
    }
  })

  it('keeps one layer of cards: the carousels are no box and no card sits in another', async () => {
    await open(viewOf({ steps: steps(fourSlots()) }))
    const cards = [...document.body.querySelectorAll('[data-slot="carousel"] [data-slot="card"]')]
    expect(cards).toHaveLength(4)
    const isBox = (el: Element) =>
      [...el.classList].some(
        (c) =>
          c === 'border' ||
          c.startsWith('rounded') ||
          c.startsWith('shadow') ||
          (c.startsWith('bg-') && c !== 'bg-transparent'),
      )
    for (const card of cards) {
      expect(card.tagName).toBe('ARTICLE')
      expect(card.querySelector('[data-slot="card"]')).toBeNull()
      expect(card.parentElement!.closest('[data-slot="card"]')).toBeNull()
      // Between the carousel and each card: plain wrappers only.
      const root = card.closest('[data-slot="carousel"]')!
      for (let el = card.parentElement; el && el !== root.parentElement; el = el.parentElement)
        expect(isBox(el)).toBe(false)
    }
  })

  it('keeps the other steps as stacked cards', async () => {
    const flow = steps([])
    flow[1]!.status = 'open'
    flow[1]!.variants = [
      variantOf({ id: 'vp', step_id: 'step-channel_plan', kind: 'channel_plan' }),
    ]
    await open(viewOf({ steps: flow.slice(0, 2) }))
    expect(carousels()).toHaveLength(0)
    expect(document.body.querySelectorAll('[data-slot="card"]')).toHaveLength(1)
  })
})

describe('groups that wait without a draft to show (D-095)', () => {
  const group = (key: string) =>
    document.body.querySelector<HTMLElement>(`[data-carousel="${key}"]`)

  function expectEmptyGroup(key: string, count: string, name?: string) {
    const el = group(key)!
    expect(el).not.toBeNull()
    expect(el.querySelector('[data-slot="carousel"]')).toBeNull()
    expect(el.querySelector('[data-empty]')!.textContent?.trim()).toBe(
      'Bekleyen taslak yok; adımı yeniden üretin.',
    )
    expect(el.querySelector('[data-pending]')!.textContent?.trim()).toBe(count)
    expect(el.querySelector('[data-pending]')!.getAttribute('aria-label')).toBe(
      name ? `${name}: ${count} onay kaldı` : `${count} onay kaldı`,
    )
    // No arrows, no position, no card.
    expect(el.querySelector('button')).toBeNull()
    expect(el.querySelector('[data-position]')).toBeNull()
    expect(el.querySelector('article')).toBeNull()
  }

  it('shows the label, the badge and one line for a channel whose only variants were rejected', async () => {
    const list = [text('mail-tr', 'tr', 'email'), text('sms-tr', 'tr', 'sms', 'rejected')]
    await open(viewOf({ steps: steps(list) }))

    expect(carousel('email')).not.toBeNull()
    expect(group('sms')!.querySelector('[data-carousel-label]')!.textContent?.trim()).toBe('SMS')
    expectEmptyGroup('sms', '1', 'SMS')
  })

  it('does the same for a channel with a slot the step has not produced yet', async () => {
    const flow = steps([text('mail-tr', 'tr', 'email')])
    flow[2]!.missing = [
      { language: 'tr', channel: 'sms' },
      { language: 'en', channel: 'sms' },
    ]
    await open(viewOf({ steps: flow }))

    expectEmptyGroup('sms', '2', 'SMS')
    expect(countOf('email')).toBe('1')
  })

  it('leaves out a channel that has no slot at all', async () => {
    await open(viewOf({ steps: steps([text('mail-tr', 'tr', 'email')]) }))
    expect(group('sms')).toBeNull()
  })

  it('shows no empty line once the step is approved', async () => {
    const list = [
      text('mail-tr', 'tr', 'email', 'approved'),
      text('sms-tr', 'tr', 'sms', 'rejected'),
    ]
    await open(viewOf({ status: 'scheduled', steps: steps(list, 'approved') }))
    expect(group('sms')).toBeNull()
    expect(document.body.querySelector('[data-empty]')).toBeNull()
  })

  it('does it for the schedule step too, without a visible label', async () => {
    const list = [slot('tr-ist', 'tr', 'Europe/Istanbul', 'rejected')]
    await open(viewOf({ steps: scheduleSteps(list) }))
    expect(group('schedule')!.querySelector('[data-carousel-label]')).toBeNull()
    expectEmptyGroup('schedule', '1')
  })
})

describe('schedule step carousel (D-095)', () => {
  const slots = () => [
    slot('tr-ist', 'tr', 'Europe/Istanbul'),
    slot('en-lon', 'en', 'Europe/London'),
    slot('tr-ber', 'tr', 'Europe/Berlin'),
  ]
  const flow = (variants: VariantView[]) => scheduleSteps(variants)

  it('shows one slide per slot, ordered by language then time zone, with a position and a badge', async () => {
    await open(viewOf({ steps: flow(slots()) }))

    expect(carousels().map((c) => c.getAttribute('data-carousel'))).toEqual(['schedule'])
    expect(carousel('schedule')!.getAttribute('aria-label')).toBe('Varyantlar')
    expect(carousel('schedule')!.querySelector('[data-carousel-label]')).toBeNull()
    expect(titlesOf('schedule')).toEqual([
      'İngilizce · Europe/London',
      'Türkçe · Europe/Berlin',
      'Türkçe · Europe/Istanbul',
    ])
    expect(positionOf('schedule')).toBe('1 / 3')
    expect(shownIn('schedule')).toHaveLength(1)
    expect(countOf('schedule')).toBe('3')
    expect(badgeOf('schedule')!.getAttribute('aria-label')).toBe('3 onay kaldı')
    // Schedule slots have no edit button.
    expect(slidesOf('schedule')[0]!.textContent).not.toContain('Düzenle')
  })

  it('moves with the arrows and on to the next waiting slot after an approval, wrapping around', async () => {
    await open(viewOf({ steps: flow(slots()) }))
    const [ist, lon, ber] = slots() as [VariantView, VariantView, VariantView]
    await next('schedule')
    await next('schedule')
    expect(positionOf('schedule')).toBe('3 / 3')

    await approve(
      'schedule',
      2,
      [ist, lon, ber].map((v) => (v === ist ? approved(v) : v)),
      flow,
    )
    expect(proposal.approveVariant).toHaveBeenCalledWith('tr-ist')
    expect(positionOf('schedule')).toBe('1 / 3')
    expect(countOf('schedule')).toBe('2')

    await approve('schedule', 0, [approved(ist), approved(lon), ber], flow)
    expect(positionOf('schedule')).toBe('2 / 3')
    expect(countOf('schedule')).toBe('1')

    await approve('schedule', 1, [approved(ist), approved(lon), approved(ber)], flow)
    expect(positionOf('schedule')).toBe('2 / 3')
    expect(badgeOf('schedule')).toBeNull()
  })

  it('shows the approved variants of an approved schedule step in the carousel, without a badge', async () => {
    const list = [
      slot('en-lon', 'en', 'Europe/London', 'approved'),
      slot('tr-ist', 'tr', 'Europe/Istanbul', 'approved'),
      slot('tr-old', 'tr', 'Europe/Istanbul', 'rejected'),
    ]
    await open(viewOf({ status: 'scheduled', steps: scheduleSteps(list, 'approved') }))

    expect(titlesOf('schedule')).toEqual(['İngilizce · Europe/London', 'Türkçe · Europe/Istanbul'])
    expect(positionOf('schedule')).toBe('1 / 2')
    expect(badgeOf('schedule')).toBeNull()
    await next('schedule')
    expect(positionOf('schedule')).toBe('2 / 2')
  })
})
