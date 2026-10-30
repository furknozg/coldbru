import { DOMWrapper, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, NetworkError } from '@/api/errors'
import * as analytics from '@/api/analytics/service'
import * as proposal from '@/api/proposal/service'
import { resetToasts } from '@/features/campaigns/composables/useToasts'
import type { CampaignView, StepKind, StepView } from '@/features/campaigns/types'
import { setViewportWide } from '@/__tests__/setup'
import {
  FakeEventSource,
  audienceContent,
  dialog,
  mountApp,
  stepOf,
  variantOf,
  viewOf,
} from '@/__tests__/helpers'

vi.mock('@/api/proposal/service')
vi.mock('@/api/analytics/service')

let wrapper: Awaited<ReturnType<typeof mountApp>>['wrapper'] | undefined

beforeEach(() => {
  FakeEventSource.install()
  vi.mocked(proposal.getLlmMode).mockResolvedValue('anthropic')
  vi.mocked(proposal.listCampaigns).mockResolvedValue([])
  vi.mocked(analytics.getCampaignSummary).mockResolvedValue(null)
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  document.body.innerHTML = ''
  resetToasts()
  vi.useRealTimers()
  vi.resetAllMocks()
})

async function open(view: CampaignView) {
  vi.mocked(proposal.getCampaign).mockResolvedValue(view)
  const mounted = await mountApp(`/campaigns/${view.id}`)
  wrapper = mounted.wrapper
  return mounted
}

/** The text of an element with its text nodes separated by a space, as a reader would hear it. */
function words(el: Element) {
  const parts: string[] = []
  const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT)
  while (walker.nextNode()) {
    const text = walker.currentNode.textContent!.trim()
    if (text) parts.push(text)
  }
  return parts.join(' ')
}

/** A button anywhere on the page, dialogs and drawers included (they render into the body). */
function button(text: string, root: ParentNode = document.body) {
  const found = [...root.querySelectorAll('button')].find((b) => words(b) === text)
  if (!found) throw new Error(`no button "${text}"`)
  return new DOMWrapper(found)
}
const hasButton = (text: string, root: ParentNode = document.body) =>
  [...root.querySelectorAll('button')].some((b) => words(b) === text)

const chat = () => document.body.querySelector<HTMLElement>('ol[aria-label="Kampanya sohbeti"]')!
/** The block of the open step, with its variants and actions. */
const openStep = () => document.body.querySelector<HTMLElement>('[data-testid="open-step"]')
/** The accordion of an approved step. */
const accordion = (stepId: string) =>
  document.body.querySelector<HTMLElement>(`li[data-step-id="${stepId}"]`)
const trigger = (stepId: string) => accordion(stepId)!.querySelector('button')!

async function expand(stepId: string) {
  await new DOMWrapper(trigger(stepId)).trigger('click')
  await flushPromises()
}

/** The flow up to and including `upTo`; earlier steps are approved, `upTo` is open. */
function flowUpTo(upTo: StepKind, open: Partial<StepView> = {}): StepView[] {
  const order: StepKind[] = ['audience', 'channel_plan', 'content', 'schedule']
  return order.slice(0, order.indexOf(upTo) + 1).map((kind) =>
    kind === upTo
      ? stepOf(kind, { id: `step-${kind}`, status: 'open', ...open })
      : stepOf(kind, {
          id: `step-${kind}`,
          status: 'approved',
          approved_at: '2026-10-30T10:00:01Z',
        }),
  )
}

const audienceDraft = (stepId = 'step-audience') =>
  variantOf({ id: 'va', step_id: stepId, kind: 'audience', content: audienceContent })

const planContent = {
  mode: 'primary_fallback',
  primary: 'email',
  rationale: 'E-posta daha çok kişiye ulaşıyor.',
  recipients: { email: 35, sms: 13, total: 48 },
  by_language: [
    { language: 'tr', email: 29, sms: 13 },
    { language: 'en', email: 6, sms: 0 },
  ],
}

const emailStored = {
  subject: 'Your cart',
  preheader: 'Pre',
  headline: 'Head',
  paragraphs: ['One', 'Two'],
  cta_label: 'Go',
  estimated_cost_usd: '0.0060',
}

function contentSteps() {
  const steps = flowUpTo('content')
  steps[2]!.variants = [
    variantOf({
      id: 'vs',
      step_id: 'step-content',
      kind: 'content',
      language: 'tr',
      channel: 'sms',
      content: {
        text: 'Merhaba {{first_name}}, %30 indirim <b>seni</b> bekliyor',
        encoding: 'ucs2',
        segments: 2,
        rendered_length: 75,
        estimated_cost_usd: '1.0400',
      },
    }),
    variantOf({
      id: 've',
      step_id: 'step-content',
      kind: 'content',
      language: 'en',
      channel: 'email',
      content: emailStored,
    }),
  ]
  return steps
}

const rendered = (subject: string, body: string) => ({
  subject,
  html: `<!DOCTYPE html><html><body><p>${body}</p></body></html>`,
  text: body,
})

describe('steps in the chat', () => {
  it('shows the open audience step inline with its proposal and no step panel beside the chat', async () => {
    const steps = flowUpTo('audience')
    steps[0]!.variants = [audienceDraft()]
    await open(viewOf({ steps }))

    expect(document.body.querySelector('[data-testid="step-panel"]')).toBeNull()
    expect(wrapper!.findAll('aside')).toHaveLength(1) // the campaign list only
    const block = openStep()!
    expect(chat().contains(block)).toBe(true)
    expect(words(block.querySelector('header')!)).toBe('Kitle Açık')
    expect(block.querySelector('pre')!.textContent).toBe(audienceContent.dsl)
    expect(block.querySelector('pre')!.className).toContain('font-mono')
    expect(block.textContent).toContain('48')
    const rows = [...block.querySelectorAll('table')[0]!.querySelectorAll('tr')].map((tr) =>
      words(tr),
    )
    expect(rows).toEqual(['İkisi de 22', 'Yalnız e-posta 13', 'Yalnız SMS 13', 'Ulaşılamaz 3'])
    const breakdown = block.querySelectorAll('table')[1]!.textContent!
    expect(breakdown).toContain('Türkçe')
    expect(breakdown).toContain('Europe/Istanbul')
    expect(breakdown).toContain('İngilizce')
    expect(breakdown).toContain('Europe/London')
    // The open step sits after the cards and before the composer.
    const kinds = [...chat().querySelectorAll(':scope > li')].map((li) =>
      li.getAttribute('data-kind'),
    )
    expect(kinds.slice(-2)).toEqual(['openStep', 'campaignActions'])
  })

  it('folds approved steps into accordions on their approval card and unfolds them on demand', async () => {
    const steps = flowUpTo('channel_plan')
    steps[0]!.variants = [
      variantOf({
        id: 'va',
        step_id: 'step-audience',
        kind: 'audience',
        status: 'approved',
        content: audienceContent,
      }),
    ]
    steps[1]!.variants = [
      variantOf({
        id: 'vp',
        step_id: 'step-channel_plan',
        kind: 'channel_plan',
        content: planContent,
      }),
    ]
    await open(viewOf({ steps }))

    const folded = accordion('step-audience')!
    expect(words(trigger('step-audience'))).toContain('Kitle adımı onaylandı Onaylandı')
    expect(trigger('step-audience').getAttribute('aria-expanded')).toBe('false')
    expect(folded.querySelector('pre')).toBeNull()

    await expand('step-audience')
    expect(trigger('step-audience').getAttribute('aria-expanded')).toBe('true')
    expect(folded.querySelector('pre')!.textContent).toBe(audienceContent.dsl)
    expect(hasButton('Onayla', folded)).toBe(false)

    const text = words(openStep()!)
    expect(text).toContain('Kanal planı Açık')
    expect(text).toContain('Mod Birincil + yedek')
    expect(text).toContain('Birincil kanal E-posta')
    expect(text).toContain('E-posta daha çok kişiye ulaşıyor.')
    expect(text).toContain('Türkçe 29 13')
    expect(text).toContain('İngilizce 6 0')
    expect(text).toContain('Toplam 48 35 13')
  })

  it('shows SMS text with encoding, segments and cost, and an email as its rendered preview', async () => {
    vi.mocked(proposal.previewVariant).mockResolvedValue(
      rendered('Your cart Alex', 'Head One Two <b>Go</b>'),
    )
    await open(viewOf({ steps: contentSteps() }))

    const cards = [...openStep()!.querySelectorAll('article')]
    expect(cards[0]!.textContent).toContain(
      'Merhaba {{first_name}}, %30 indirim <b>seni</b> bekliyor',
    )
    expect(cards[0]!.querySelector('b')).toBeNull()
    expect(cards[0]!.textContent).toContain('UCS-2')
    expect(words(cards[0]!)).toContain('Segment 2')
    expect(cards[0]!.textContent).toContain('$1,04')

    expect(proposal.previewVariant).toHaveBeenCalledWith('ve', undefined)
    const email = cards[1]!
    expect(email.querySelector('[data-preview-subject]')!.textContent).toBe('Your cart Alex')
    const frame = email.querySelector('iframe')!
    expect(frame.getAttribute('sandbox')).toBe('')
    expect(frame.getAttribute('srcdoc')).toContain('<p>Head One Two <b>Go</b></p>')
    // The email's HTML lives in the frame, never in the page.
    expect(email.querySelector('b')).toBeNull()
    expect(email.textContent).toContain('$0,006')
    expect(hasButton('Onayla', cards[0]!)).toBe(true)
    expect(hasButton('Düzenle', cards[0]!)).toBe(false)
    expect(hasButton('Düzenle', email)).toBe(true)
  })

  it('falls back to the email blocks as text when the preview cannot be fetched', async () => {
    vi.mocked(proposal.previewVariant).mockRejectedValueOnce(new NetworkError(new TypeError('x')))
    await open(viewOf({ steps: contentSteps() }))

    const email = openStep()!.querySelectorAll('article')[1]!
    expect(email.querySelector('iframe')).toBeNull()
    expect(email.querySelector('[role="alert"]')!.textContent).toContain('Önizleme alınamadı')
    const blocks = email.querySelector('[data-email-blocks]')!
    expect(blocks.textContent).toContain('Your cart')
    expect(blocks.textContent).toContain('One')
    expect(blocks.textContent).toContain('Two')

    vi.mocked(proposal.previewVariant).mockResolvedValueOnce(rendered('Your cart Alex', 'ok'))
    await button('Yeniden dene', email).trigger('click')
    await flushPromises()
    expect(email.querySelector('iframe')).not.toBeNull()
  })

  it('shows each schedule slot in its own time zone with the rationale', async () => {
    const steps = flowUpTo('schedule')
    steps[3]!.variants = [
      variantOf({
        id: 'v1',
        step_id: 'step-schedule',
        kind: 'schedule',
        language: 'tr',
        timezone: 'Europe/Istanbul',
        content: { send_at: '2026-11-01T06:30:00Z', rationale: 'Pazar sabahı' },
      }),
      variantOf({
        id: 'v2',
        step_id: 'step-schedule',
        kind: 'schedule',
        language: 'en',
        timezone: 'Europe/London',
        content: { send_at: null, rationale: 'Hemen gitsin' },
      }),
    ]
    await open(viewOf({ steps }))

    const [tr, en] = [...openStep()!.querySelectorAll('article')]
    expect(tr!.textContent).toContain('Türkçe · Europe/Istanbul')
    expect(tr!.textContent).toContain('09:30')
    expect(tr!.textContent).toContain('Pazar sabahı')
    expect(en!.textContent).toContain('Hemen gönder')
    expect(hasButton('Düzenle', tr!)).toBe(false)
  })

  it('shows the cost summary under the content step', async () => {
    await open(
      viewOf({
        steps: flowUpTo('content'),
        cost: { sms_usd: '1.0400', email_usd: '0.0330', total_usd: '1.0730' },
      }),
    )

    const text = words(openStep()!.querySelector('[data-cost]')!)
    expect(text).toContain('E-posta $0,033')
    expect(text).toContain('SMS $1,04')
    expect(text).toContain('Toplam $1,073')
  })

  it('shows the title of the campaign on top and takes a new one from the stream', async () => {
    await open(viewOf({ title: 'İstanbul sepet hatırlatması', steps: flowUpTo('audience') }))

    const title = () => document.body.querySelector('[data-campaign-title]')!
    expect(title().textContent!.trim()).toBe('İstanbul sepet hatırlatması')
    expect(wrapper!.get('aside li').text()).toContain('İstanbul sepet hatırlatması')
    // The brief is the first message of the chat, not the title.
    expect(chat().querySelector('li[data-kind="brief"]')!.textContent).toContain(
      'İstanbul için %30 indirim hatırlatması',
    )

    FakeEventSource.last.emit('campaign_titled', {
      at: '2026-10-30T10:00:02Z',
      campaign_id: 'c1',
      title: 'Sepet hatırlatması',
    })
    await flushPromises()
    expect(title().textContent!.trim()).toBe('Sepet hatırlatması')
    expect(wrapper!.get('aside li').text()).toContain('Sepet hatırlatması')
  })

  it('falls back to the brief as title while the campaign has none', async () => {
    await open(viewOf({ steps: [] }))
    expect(document.body.querySelector('[data-campaign-title]')!.textContent!.trim()).toBe(
      'İstanbul için %30 indirim hatırlatması',
    )
  })
})

describe('approving and rejecting', () => {
  async function openAudience() {
    const steps = flowUpTo('audience')
    steps[0]!.variants = [audienceDraft()]
    return open(viewOf({ steps }))
  }

  it('approves a variant and shows the view the server answers with', async () => {
    await openAudience()
    const next = flowUpTo('channel_plan')
    next[1]!.variants = [
      variantOf({
        id: 'vp',
        step_id: 'step-channel_plan',
        kind: 'channel_plan',
        content: planContent,
      }),
    ]
    vi.mocked(proposal.approveVariant).mockResolvedValue(viewOf({ steps: next }))

    await button('Onayla', openStep()!).trigger('click')
    await flushPromises()

    expect(proposal.approveVariant).toHaveBeenCalledWith('va')
    expect(words(openStep()!)).toContain('Kanal planı Açık')
    expect(openStep()!.textContent).toContain('E-posta daha çok kişiye ulaşıyor.')
    expect(accordion('step-audience')).not.toBeNull()
  })

  it('needs a reason to reject, sends it trimmed and closes the dialog', async () => {
    await openAudience()
    vi.mocked(proposal.rejectVariant).mockResolvedValue(viewOf({ steps: flowUpTo('audience') }))

    await button('Reddet', openStep()!).trigger('click')
    await flushPromises()
    expect(dialog()).not.toBeNull()
    const confirm = () => button('Reddet', dialog()!)
    expect(confirm().element.hasAttribute('disabled')).toBe(true)

    const reason = new DOMWrapper(dialog()!.querySelector('textarea')!)
    await reason.setValue('   ')
    expect(confirm().element.hasAttribute('disabled')).toBe(true)
    await confirm().trigger('click')
    expect(proposal.rejectVariant).not.toHaveBeenCalled()

    await reason.setValue('  Kitle çok geniş  ')
    expect(confirm().element.hasAttribute('disabled')).toBe(false)
    await confirm().trigger('click')
    await flushPromises()

    expect(proposal.rejectVariant).toHaveBeenCalledWith('va', 'Kitle çok geniş')
    expect(dialog()).toBeNull()
  })

  it('limits the reason to 1000 characters', async () => {
    await openAudience()
    await button('Reddet', openStep()!).trigger('click')
    await flushPromises()

    await new DOMWrapper(dialog()!.querySelector('textarea')!).setValue('x'.repeat(1001))

    expect(dialog()!.textContent).toContain('1.001/1.000')
    expect(button('Reddet', dialog()!).element.hasAttribute('disabled')).toBe(true)
  })

  it('shows a conflict as a notice and fetches the view again', async () => {
    await openAudience()
    vi.mocked(proposal.approveVariant).mockRejectedValue(
      ApiError.from(409, {
        code: 'step_not_open',
        title: 'The step is not open',
        detail: 'Adım artık açık değil.',
      }),
    )
    const before = vi.mocked(proposal.getCampaign).mock.calls.length

    await button('Onayla', openStep()!).trigger('click')
    await flushPromises()

    const toast = document.body.querySelector('[role="alert"]')!
    expect(toast.textContent).toContain('Adım artık açık değil.')
    expect(vi.mocked(proposal.getCampaign).mock.calls.length).toBe(before + 1)
  })

  it('offers no approve, reject or reopen once the campaign is not a draft', async () => {
    const steps = flowUpTo('schedule', { status: 'approved' })
    steps[3]!.variants = [
      variantOf({
        id: 'v1',
        step_id: 'step-schedule',
        kind: 'schedule',
        status: 'approved',
        language: 'tr',
        timezone: 'Europe/Istanbul',
        content: { send_at: null, rationale: 'x' },
      }),
    ]
    await open(viewOf({ status: 'scheduled', steps }))

    expect(openStep()).toBeNull()
    await expand('step-schedule')
    expect(hasButton('Onayla', chat())).toBe(false)
    expect(hasButton('Reddet', chat())).toBe(false)
    expect(hasButton('Yeniden aç', chat())).toBe(false)
    expect(hasButton('İptal et', chat())).toBe(true)
  })
})

describe('editing', () => {
  it('edits the audience DSL and shows the 422 problems under the field', async () => {
    const steps = flowUpTo('audience')
    steps[0]!.variants = [audienceDraft()]
    await open(viewOf({ steps }))
    vi.mocked(proposal.editVariant).mockRejectedValueOnce(
      ApiError.from(422, {
        code: 'invalid_content',
        title: 'Invalid content',
        detail: 'The audience rule is not valid.',
        problems: ['unknown field "citty"', 'line 1: expected an expression'],
      }),
    )

    await button('Düzenle', openStep()!).trigger('click')
    const area = new DOMWrapper(openStep()!.querySelector('textarea')!)
    expect((area.element as HTMLTextAreaElement).value).toBe(audienceContent.dsl)
    await area.setValue('citty === "İstanbul"')
    await openStep()!
      .querySelector('form')!
      .dispatchEvent(new Event('submit', { cancelable: true }))
    await flushPromises()

    expect(proposal.editVariant).toHaveBeenCalledWith('va', { dsl: 'citty === "İstanbul"' })
    const alert = openStep()!.querySelector('[role="alert"]')!
    expect(alert.textContent).toContain('The audience rule is not valid.')
    expect(alert.textContent).toContain('unknown field "citty"')
    expect(alert.textContent).toContain('line 1: expected an expression')
    // A 422 belongs to the form: no toast, and the form stays open with the text.
    expect(document.body.querySelectorAll('[role="alert"]')).toHaveLength(1)
    expect((openStep()!.querySelector('textarea') as HTMLTextAreaElement).value).toBe(
      'citty === "İstanbul"',
    )

    // Fixed: the edit goes through, the view is fetched again and the form closes.
    vi.mocked(proposal.editVariant).mockResolvedValueOnce(variantOf())
    const before = vi.mocked(proposal.getCampaign).mock.calls.length
    await area.setValue('city === "İzmir"')
    await openStep()!
      .querySelector('form')!
      .dispatchEvent(new Event('submit', { cancelable: true }))
    await flushPromises()

    expect(proposal.editVariant).toHaveBeenLastCalledWith('va', { dsl: 'city === "İzmir"' })
    expect(vi.mocked(proposal.getCampaign).mock.calls.length).toBe(before + 1)
    expect(openStep()!.querySelector('form')).toBeNull()
  })

  it('edits the channel plan with mode, primary channel and rationale', async () => {
    const steps = flowUpTo('channel_plan')
    steps[1]!.variants = [
      variantOf({
        id: 'vp',
        step_id: 'step-channel_plan',
        kind: 'channel_plan',
        content: planContent,
      }),
    ]
    await open(viewOf({ steps }))
    vi.mocked(proposal.editVariant).mockResolvedValue(variantOf())

    await button('Düzenle', openStep()!).trigger('click')
    const [mode, primary] = [...openStep()!.querySelectorAll('form select')] as HTMLSelectElement[]
    expect([...mode!.options].map((o) => o.text)).toEqual([
      'Tek kanal',
      'Birincil + yedek',
      'İki kanal birden',
    ])
    expect(mode!.value).toBe('primary_fallback')
    expect(primary!.value).toBe('email')
    await new DOMWrapper(mode!).setValue('single')
    await new DOMWrapper(primary!).setValue('sms')
    await new DOMWrapper(openStep()!.querySelector('form textarea')!).setValue('  Yalnız SMS  ')
    await openStep()!
      .querySelector('form')!
      .dispatchEvent(new Event('submit', { cancelable: true }))
    await flushPromises()

    expect(proposal.editVariant).toHaveBeenCalledWith('vp', {
      mode: 'single',
      primary: 'sms',
      rationale: 'Yalnız SMS',
    })
  })

  it('edits an email next to its live preview, debounced, and saves the blocks', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    vi.mocked(proposal.previewVariant).mockImplementation(async (_id, content) =>
      rendered(
        `${content ? content.subject : 'Your cart'} Alex`,
        content ? (content.paragraphs as string[]).join(' | ') : 'stored',
      ),
    )
    await open(viewOf({ steps: contentSteps() }))
    const email = () => openStep()!.querySelectorAll('article')[1]!
    expect(email().querySelector('iframe')!.getAttribute('srcdoc')).toContain('stored')
    const storedCalls = vi.mocked(proposal.previewVariant).mock.calls.length

    await button('Düzenle', email()).trigger('click')
    await flushPromises()
    const form = () => email().querySelector('form')!
    const field = (name: string) =>
      new DOMWrapper(form().querySelector<HTMLInputElement>(`[name="${name}"]`)!)
    expect((field('subject').element as HTMLInputElement).value).toBe('Your cart')
    expect(form().querySelectorAll('textarea[name="paragraph"]')).toHaveLength(2)
    // The form opens with a preview of what it holds, then waits for the typing to pause.
    expect(proposal.previewVariant).toHaveBeenCalledTimes(storedCalls + 1)
    expect(vi.mocked(proposal.previewVariant).mock.lastCall![1]).toMatchObject({
      subject: 'Your cart',
      paragraphs: ['One', 'Two'],
    })

    await field('subject').setValue('Your cart is waiting')
    await button('Paragraf ekle', form()).trigger('click')
    const paragraphs = () => [...form().querySelectorAll<HTMLTextAreaElement>('textarea')]
    expect(paragraphs()).toHaveLength(3)
    await new DOMWrapper(paragraphs()[2]!).setValue('Three')
    expect(proposal.previewVariant).toHaveBeenCalledTimes(storedCalls + 1)
    await vi.advanceTimersByTimeAsync(399)
    expect(proposal.previewVariant).toHaveBeenCalledTimes(storedCalls + 1)
    await vi.advanceTimersByTimeAsync(1)
    await flushPromises()
    expect(proposal.previewVariant).toHaveBeenCalledTimes(storedCalls + 2)
    expect(vi.mocked(proposal.previewVariant).mock.lastCall).toEqual([
      've',
      {
        subject: 'Your cart is waiting',
        preheader: 'Pre',
        headline: 'Head',
        paragraphs: ['One', 'Two', 'Three'],
        cta_label: 'Go',
      },
    ])
    expect(form().querySelector('[data-preview-subject]')!.textContent).toBe(
      'Your cart is waiting Alex',
    )
    expect(form().querySelector('iframe')!.getAttribute('srcdoc')).toContain('One | Two | Three')

    // A preview the validators refuse shows its lines under the fields; the last preview stays.
    vi.mocked(proposal.previewVariant).mockRejectedValueOnce(
      ApiError.from(422, {
        code: 'invalid_content',
        title: 'Invalid content',
        detail: 'The content did not pass validation.',
        problems: ['email.subject is 91 characters long; the limit is 80'],
      }),
    )
    await field('subject').setValue('x'.repeat(91))
    await vi.advanceTimersByTimeAsync(400)
    await flushPromises()
    expect(form().querySelector('[role="alert"]')!.textContent).toContain(
      'email.subject is 91 characters long',
    )
    expect(form().querySelector('iframe')!.getAttribute('srcdoc')).toContain('One | Two | Three')

    // Remove a paragraph and save: the blocks go to the edit endpoint, trimmed, and the form closes.
    await new DOMWrapper(form().querySelector('[aria-label="3. paragrafı kaldır"]')!).trigger(
      'click',
    )
    await field('subject').setValue('  Final subject ')
    vi.mocked(proposal.editVariant).mockResolvedValueOnce(variantOf())
    await form().dispatchEvent(new Event('submit', { cancelable: true }))
    await flushPromises()

    expect(proposal.editVariant).toHaveBeenCalledWith('ve', {
      subject: 'Final subject',
      preheader: 'Pre',
      headline: 'Head',
      paragraphs: ['One', 'Two'],
      cta_label: 'Go',
    })
    expect(email().querySelector('form')).toBeNull()
  })

  it('shows the 422 of a saved email under the fields and keeps the form open', async () => {
    await open(viewOf({ steps: contentSteps() }))
    const email = () => openStep()!.querySelectorAll('article')[1]!
    await button('Düzenle', email()).trigger('click')
    vi.mocked(proposal.editVariant).mockRejectedValueOnce(
      ApiError.from(422, {
        code: 'invalid_content',
        title: 'Invalid content',
        detail: 'The content did not pass validation.',
        problems: ['email.cta_label must not be empty'],
      }),
    )

    await new DOMWrapper(email().querySelector('[name="cta_label"]')!).setValue('')
    await email()
      .querySelector('form')!
      .dispatchEvent(new Event('submit', { cancelable: true }))
    await flushPromises()

    expect(email().querySelector('[role="alert"]')!.textContent).toContain(
      'email.cta_label must not be empty',
    )
    expect(email().querySelector('form')).not.toBeNull()
    expect(document.body.querySelectorAll('[role="alert"]')).toHaveLength(1)
  })

  it('can leave the edit form without sending anything', async () => {
    const steps = flowUpTo('audience')
    steps[0]!.variants = [audienceDraft()]
    await open(viewOf({ steps }))

    await button('Düzenle', openStep()!).trigger('click')
    await button('Vazgeç', openStep()!).trigger('click')

    expect(openStep()!.querySelector('form')).toBeNull()
    expect(proposal.editVariant).not.toHaveBeenCalled()
  })
})

describe('questions of the agent', () => {
  const question = {
    id: 'q1',
    campaign_id: 'c1',
    step_id: 'step-audience',
    kind: 'question' as const,
    body: 'Hangi **müşteriler**?\n\n- sadece aktifler\n- herkes',
    options: ['Sadece aktifler', 'Herkes'],
    created_at: '2026-10-30T10:00:03Z',
  }

  function askedView() {
    const steps = flowUpTo('audience', { pending_question: question })
    return viewOf({ steps, messages: [question] })
  }

  it('shows the question as markdown with its options as buttons and answers with one', async () => {
    await open(askedView())
    const answered = viewOf({ steps: flowUpTo('audience', { generating: true }) })
    vi.mocked(proposal.answerQuestion).mockResolvedValue(answered)

    const card = chat().querySelector('li[data-kind="question"]')!
    expect(card.getAttribute('data-pending')).toBe('true')
    expect(card.textContent).toContain('Kitle için soru')
    expect(card.querySelector('strong')!.textContent).toBe('müşteriler')
    expect(card.querySelectorAll('li')).toHaveLength(2)
    expect(hasButton('Sadece aktifler', card)).toBe(true)
    expect(hasButton('Herkes', card)).toBe(true)

    await button('Herkes', card).trigger('click')
    await flushPromises()

    expect(proposal.answerQuestion).toHaveBeenCalledWith('q1', 'Herkes')
    expect(openStep()!.textContent).toContain('Üretiliyor…')
  })

  it('sends free text from the composer as the answer while a question waits', async () => {
    await open(askedView())
    vi.mocked(proposal.answerQuestion).mockResolvedValue(viewOf({ steps: flowUpTo('audience') }))

    const box = () => wrapper!.get('textarea')
    expect(box().attributes('placeholder')).toContain('Cevabınızı yazın')
    expect(hasButton('Cevapla')).toBe(true)
    await box().setValue('  Son 90 günde alışveriş yapanlar  ')
    await wrapper!.get('form').trigger('submit')
    await flushPromises()

    expect(proposal.answerQuestion).toHaveBeenCalledWith('q1', 'Son 90 günde alışveriş yapanlar')
    expect(proposal.addContext).not.toHaveBeenCalled()
    expect((box().element as HTMLTextAreaElement).value).toBe('')
    // Answered: the composer is a note box again.
    expect(box().attributes('placeholder')).toContain('bağlam ekleyin')
    expect(hasButton('Not ekle')).toBe(true)
  })

  it('shows a 422 of the answer next to the composer', async () => {
    await open(askedView())
    vi.mocked(proposal.answerQuestion).mockRejectedValue(
      ApiError.from(422, {
        code: 'invalid_request',
        title: 'Invalid request',
        detail: 'The request body does not match the expected shape.',
        problems: ['answer: must be at most 1000 characters (got 1001)'],
      }),
    )

    await wrapper!.get('textarea').setValue('x')
    await wrapper!.get('form').trigger('submit')
    await flushPromises()

    expect(wrapper!.get('form [role="alert"]').text()).toContain('answer: must be at most 1000')
  })

  it('offers no buttons on an answered question and shows the answer as the marketer’s message', async () => {
    const answer = {
      id: 'a1',
      campaign_id: 'c1',
      step_id: 'step-audience',
      kind: 'answer' as const,
      body: 'Herkes',
      reply_to: 'q1',
      created_at: '2026-10-30T10:00:04Z',
    }
    await open(viewOf({ steps: flowUpTo('audience'), messages: [question, answer] }))

    const card = chat().querySelector('li[data-kind="question"]')!
    expect(card.getAttribute('data-pending')).toBeNull()
    expect(hasButton('Herkes', card)).toBe(false)
    const reply = chat().querySelector('li[data-kind="answer"]')!
    expect(reply.className).toContain('justify-end')
    expect(reply.textContent).toContain('Cevap')
    expect(reply.textContent).toContain('Herkes')
    expect(wrapper!.get('textarea').attributes('placeholder')).toContain('bağlam ekleyin')
  })
})

describe('summaries of the agent', () => {
  it('renders a summary as sanitised markdown and the brief as plain text', async () => {
    const summary = {
      id: 'm1',
      campaign_id: 'c1',
      step_id: 'step-audience',
      kind: 'summary' as const,
      body: '**48 kişi** seçildi:\n\n- İstanbul\n- son 30 gün\n\n<script>alert(1)</script> [site](javascript:alert(2)) [ok](https://x.example)',
      created_at: '2026-10-30T10:00:05Z',
    }
    await open(viewOf({ steps: flowUpTo('audience'), messages: [summary] }))

    const card = chat().querySelector('li[data-kind="summary"]')!
    expect(card.textContent).toContain('Kitle özeti')
    expect(card.querySelector('strong')!.textContent).toBe('48 kişi')
    expect([...card.querySelectorAll('li')].map((li) => li.textContent)).toEqual([
      'İstanbul',
      'son 30 gün',
    ])
    expect(card.querySelector('script')).toBeNull()
    const links = [...card.querySelectorAll('a')]
    expect(links).toHaveLength(1)
    expect(links[0]!.getAttribute('href')).toBe('https://x.example')
    expect(links[0]!.getAttribute('rel')).toBe('noopener noreferrer')
    expect(links[0]!.getAttribute('target')).toBe('_blank')
    expect(chat().querySelector('li[data-kind="brief"] [data-markdown]')).toBeNull()
  })
})

describe('failure and recovery actions', () => {
  it('shows why a generation failed and regenerates the step', async () => {
    const steps = flowUpTo('content', {
      failed: true,
      failure_reason: 'Model 3 denemede geçerli çıktı vermedi',
    })
    await open(viewOf({ steps }))
    vi.mocked(proposal.regenerateStep).mockResolvedValue(
      viewOf({ steps: flowUpTo('content', { generating: true }) }),
    )

    expect(openStep()!.textContent).toContain('Üretim başarısız oldu.')
    expect(openStep()!.textContent).toContain('Model 3 denemede geçerli çıktı vermedi')
    await button('Yeniden üret', openStep()!).trigger('click')
    await flushPromises()

    expect(proposal.regenerateStep).toHaveBeenCalledWith('step-content')
    expect(openStep()!.textContent).toContain('Üretiliyor…')
    expect(openStep()!.textContent).not.toContain('Yeniden üret')
  })

  it('restarts a drafting campaign that has no steps', async () => {
    await open(viewOf({ steps: [] }))
    vi.mocked(proposal.regenerateCampaign).mockResolvedValue(
      viewOf({ steps: flowUpTo('audience') }),
    )

    expect(openStep()).toBeNull()
    expect(chat().textContent).toContain('henüz adım oluşmadı')
    await button('Yeniden başlat', chat()).trigger('click')
    await flushPromises()

    expect(proposal.regenerateCampaign).toHaveBeenCalledWith('c1')
    expect(hasButton('Yeniden başlat', chat())).toBe(false)
    expect(openStep()).not.toBeNull()
  })

  it('reopens an approved step from its accordion after a confirmation', async () => {
    const steps = flowUpTo('channel_plan')
    steps[0]!.variants = [
      variantOf({
        id: 'va',
        step_id: 'step-audience',
        kind: 'audience',
        status: 'approved',
        content: audienceContent,
      }),
    ]
    await open(viewOf({ steps }))
    vi.mocked(proposal.reopenStep).mockResolvedValue({} as never)

    expect(hasButton('Yeniden aç', chat())).toBe(false)
    await expand('step-audience')
    await button('Yeniden aç', accordion('step-audience')!).trigger('click')
    await flushPromises()

    expect(dialog()!.textContent).toContain('Kitle adımı yeniden açılsın mı?')
    expect(proposal.reopenStep).not.toHaveBeenCalled()
    await button('Yeniden aç', dialog()!).trigger('click')
    await flushPromises()

    expect(proposal.reopenStep).toHaveBeenCalledWith('c1', 'audience')
  })

  it('retries a confirmation when the schedule is approved but the campaign is still a draft', async () => {
    const steps = flowUpTo('schedule', { status: 'approved' })
    await open(viewOf({ status: 'drafting', steps }))
    vi.mocked(proposal.confirmCampaign).mockResolvedValue(viewOf({ status: 'scheduled', steps }))

    await button('Onayı yeniden dene', chat()).trigger('click')
    await flushPromises()

    expect(proposal.confirmCampaign).toHaveBeenCalledWith('c1')
    expect(hasButton('Onayı yeniden dene', chat())).toBe(false)
    expect(wrapper!.get('header [data-status]').attributes('data-status')).toBe('scheduled')
  })

  it('cancels with an optional reason after a confirmation', async () => {
    await open(viewOf({ steps: flowUpTo('audience') }))
    vi.mocked(proposal.cancelCampaign).mockResolvedValue(
      viewOf({ status: 'cancelled', steps: flowUpTo('audience') }),
    )

    await button('İptal et', chat()).trigger('click')
    await flushPromises()
    await new DOMWrapper(dialog()!.querySelector('textarea')!).setValue('  Yanlış indirim  ')
    await button('Kampanyayı iptal et', dialog()!).trigger('click')
    await flushPromises()

    expect(proposal.cancelCampaign).toHaveBeenCalledWith('c1', 'Yanlış indirim')
    expect(wrapper!.get('header [data-status]').attributes('data-status')).toBe('cancelled')
    expect(hasButton('İptal et', chat())).toBe(false)
  })

  it('cancels without a reason when none is typed', async () => {
    await open(viewOf({ steps: flowUpTo('audience') }))
    vi.mocked(proposal.cancelCampaign).mockResolvedValue(viewOf({ status: 'cancelled' }))

    await button('İptal et', chat()).trigger('click')
    await flushPromises()
    await button('Kampanyayı iptal et', dialog()!).trigger('click')
    await flushPromises()

    expect(proposal.cancelCampaign).toHaveBeenCalledWith('c1', undefined)
  })
})

describe('chat behaviour', () => {
  it('unfolds the step of a card that is clicked', async () => {
    const steps = flowUpTo('channel_plan')
    steps[0]!.variants = [
      variantOf({
        id: 'va',
        step_id: 'step-audience',
        kind: 'audience',
        status: 'approved',
        content: audienceContent,
      }),
    ]
    await open(viewOf({ steps }))
    expect(trigger('step-audience').getAttribute('aria-expanded')).toBe('false')

    const card = wrapper!
      .findAll('li[data-kind="stepOpened"] button')
      .find((b) => b.text().includes('Kitle adımı açıldı'))!
    await card.trigger('click')
    await flushPromises()

    expect(trigger('step-audience').getAttribute('aria-expanded')).toBe('true')
    expect(accordion('step-audience')!.querySelector('pre')!.textContent).toBe(audienceContent.dsl)
  })

  it('shows a step that opens later inline, with the previous one folded', async () => {
    const approved = flowUpTo('audience', { status: 'approved' })
    await open(viewOf({ status: 'drafting', steps: approved }))
    expect(openStep()).toBeNull()

    const steps = flowUpTo('channel_plan')
    steps[1]!.variants = [
      variantOf({
        id: 'vp',
        step_id: 'step-channel_plan',
        kind: 'channel_plan',
        content: planContent,
      }),
    ]
    FakeEventSource.last.emit('snapshot', viewOf({ status: 'drafting', steps }))
    await flushPromises()

    expect(words(openStep()!)).toContain('Kanal planı Açık')
    expect(accordion('step-audience')).not.toBeNull()
  })
})

describe('narrow screens', () => {
  it('keeps the chat in one column and moves the list into a drawer', async () => {
    setViewportWide(false)
    const steps = flowUpTo('audience')
    steps[0]!.variants = [audienceDraft()]
    await open(viewOf({ steps }))

    expect(wrapper!.find('aside').exists()).toBe(false)
    expect(openStep()!.textContent).toContain('Kitle')
    expect(hasButton('Onayla', openStep()!)).toBe(true)

    await new DOMWrapper(
      document.body.querySelector('header [aria-label="Kampanya listesini aç"]')!,
    ).trigger('click')
    await flushPromises()
    expect(
      document.body.querySelector('[role="dialog"] [aria-label="Yeni kampanya"]'),
    ).not.toBeNull()
  })
})
