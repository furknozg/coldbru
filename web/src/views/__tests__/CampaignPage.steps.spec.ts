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
  // The charts are a lazy chunk (D-092): wait for it before looking at the page.
  await vi.dynamicImportSettled()
  await flushPromises()
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
/** The block of the open step: its accordion, then its variants as cards. */
const openStep = () => document.body.querySelector<HTMLElement>('[data-testid="open-step"]')
/** The block of a step: one accordion and, under it, the variants it shows as cards. */
const accordion = (stepId: string) =>
  document.body.querySelector<HTMLElement>(`li[data-step-id="${stepId}"]`)
const trigger = (stepId: string) =>
  accordion(stepId)!.querySelector<HTMLButtonElement>('[data-slot="accordion-trigger"]')!
/** The chat bar: the text box, its buttons and the cancel button. */
const chatBar = () => document.body.querySelector<HTMLElement>('form')!
/** The accordion at the end of the chat (retry confirmation, restart), when there is one. */
const finalActions = () => chat().querySelector<HTMLElement>('li[data-kind="campaignActions"]')
const finalTrigger = () => finalActions()!.querySelector<HTMLButtonElement>('button')!

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
    expect(words(trigger('step-audience'))).toBe('Kitle Açık')
    // The open step's accordion starts folded too (D-112).
    expect(trigger('step-audience').getAttribute('aria-expanded')).toBe('false')
    // The rule is not shown (D-096): no query text, only the count and the pies.
    expect(block.querySelector('pre')).toBeNull()
    expect(block.textContent).not.toContain(audienceContent.dsl)
    expect(block.textContent).toContain('48')
    // Three pies instead of tables (D-092): the numbers sit in their legends.
    expect(block.querySelector('table')).toBeNull()
    const pies = [...block.querySelectorAll('figure')]
    expect(pies.map((pie) => pie.querySelector('figcaption')!.textContent)).toEqual([
      'Kanal kapsaması',
      'Dil',
      'Yerel ayar',
    ])
    expect([...pies[0]!.querySelectorAll('li')].map((li) => words(li))).toEqual([
      'İkisi de 22 %43,1',
      'Yalnız e-posta 13 %25,5',
      'Yalnız SMS 13 %25,5',
      'Ulaşılamaz 3 %5,9',
    ])
    expect([...pies[1]!.querySelectorAll('li')].map((li) => words(li))).toEqual([
      'Türkçe 42 %87,5',
      'İngilizce 6 %12,5',
    ])
    expect([...pies[2]!.querySelectorAll('li')].map((li) => words(li))).toEqual([
      'Türkçe · Europe/Istanbul 42 %87,5',
      'İngilizce · Europe/London 6 %12,5',
    ])
    // The open step is the last entry of the chat; with nothing to retry or restart there
    // is no accordion of campaign actions after it.
    const kinds = [...chat().querySelectorAll(':scope > li')].map((li) =>
      li.getAttribute('data-kind'),
    )
    expect(kinds[kinds.length - 1]).toBe('step')
    expect(finalActions()).toBeNull()
  })

  it('folds an approved step into its accordion, keeps its approved variant as the card and unfolds the events on demand', async () => {
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
    expect(words(trigger('step-audience'))).toBe('Kitle Onaylandı')
    expect(trigger('step-audience').getAttribute('aria-expanded')).toBe('false')
    // Folded, the step is its approved variant; the events are inside the accordion.
    const card = folded.querySelector('article[data-slot="card"]')!
    expect(card.querySelector('pre')).toBeNull()
    expect(card.textContent).not.toContain(audienceContent.dsl)
    expect(folded.textContent).not.toContain('Kitle adımı onaylandı')

    await expand('step-audience')
    expect(trigger('step-audience').getAttribute('aria-expanded')).toBe('true')
    expect(folded.textContent).toContain('Kitle adımı açıldı')
    expect(folded.textContent).toContain('Kitle adımı onaylandı')
    expect(hasButton('Onayla', folded)).toBe(false)

    const text = words(openStep()!)
    expect(text).toContain('Kanal planı Açık')
    expect(text).toContain('Mod Birincil + yedek')
    expect(text).toContain('Birincil kanal E-posta')
    expect(text).toContain('E-posta daha çok kişiye ulaşıyor.')
    // One pie of recipients per channel instead of the table (D-092).
    expect(text).toContain('Kanal başına alıcılar')
    expect(text).toContain('E-posta 35 %72,9')
    expect(text).toContain('SMS 13 %27,1')
    expect(openStep()!.querySelector('table')).toBeNull()
  })

  it('shows SMS text with encoding, segments and cost, and an email as its rendered preview', async () => {
    vi.mocked(proposal.previewVariant).mockResolvedValue(
      rendered('Your cart Alex', 'Head One Two <b>Go</b>'),
    )
    await open(viewOf({ steps: contentSteps() }))

    // The carousel puts the email before the SMS.
    const [email, sms] = openStep()!.querySelectorAll('article')
    if (!email || !sms) throw new Error('expected an email and an SMS card')
    expect(sms.textContent).toContain('Merhaba {{first_name}}, %30 indirim <b>seni</b> bekliyor')
    expect(sms.querySelector('b')).toBeNull()
    expect(sms.textContent).toContain('UCS-2')
    expect(words(sms)).toContain('Segment 2')
    expect(sms.textContent).toContain('$1,04')

    expect(proposal.previewVariant).toHaveBeenCalledWith('ve', undefined)
    expect(email.querySelector('[data-preview-subject]')!.textContent).toBe('Your cart Alex')
    const frame = email.querySelector('iframe')!
    expect(frame.getAttribute('sandbox')).toBe('')
    expect(frame.getAttribute('srcdoc')).toContain('<p>Head One Two <b>Go</b></p>')
    // The email's HTML lives in the frame, never in the page.
    expect(email.querySelector('b')).toBeNull()
    expect(email.textContent).toContain('$0,006')
    expect(hasButton('Onayla', sms)).toBe(true)
    expect(hasButton('Düzenle', sms)).toBe(false)
    expect(hasButton('Düzenle', email)).toBe(true)
  })

  it('falls back to the email blocks as text when the preview cannot be fetched', async () => {
    vi.mocked(proposal.previewVariant).mockRejectedValueOnce(new NetworkError(new TypeError('x')))
    await open(viewOf({ steps: contentSteps() }))

    const email = openStep()!.querySelectorAll('article')[0]!
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

    // The carousel orders by language: English before Turkish.
    const [en, tr] = [...openStep()!.querySelectorAll('article')]
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

  it('shows the status in the header as muted plain text, not as a badge', async () => {
    await open(viewOf({ steps: flowUpTo('audience') }))

    const header = document.body.querySelector('section > header')!
    const status = header.querySelector('[data-status]')!
    expect(status.textContent!.trim()).toBe('Taslak')
    expect(status.getAttribute('data-status')).toBe('drafting')
    expect(status.className).toContain('text-muted-foreground')
    expect(header.querySelector('[data-slot="badge"]')).toBeNull()
    expect(boxes(header)).toEqual([])
    // Next to the title, before the date.
    expect(header.querySelector('h2')!.nextElementSibling).toBe(status)
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
    expect(finalActions()).toBeNull()
    // A scheduled campaign can still be cancelled, from the chat bar.
    expect(hasButton('İptal et', chatBar())).toBe(true)
  })
})

describe('editing', () => {
  it('offers no edit for the audience (D-096): approve and reject only, and no rule text', async () => {
    const steps = flowUpTo('audience')
    steps[0]!.variants = [audienceDraft()]
    await open(viewOf({ steps }))

    const card = openStep()!.querySelector('article')!
    expect(hasButton('Onayla', card)).toBe(true)
    expect(hasButton('Reddet', card)).toBe(true)
    expect(hasButton('Düzenle', card)).toBe(false)
    expect(card.querySelector('textarea')).toBeNull()
    expect(card.textContent).not.toContain('city ===')
    expect(card.querySelector('[aria-label="Kitle kuralı"]')).toBeNull()
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
    const email = () => openStep()!.querySelectorAll('article')[0]!
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
    const email = () => openStep()!.querySelectorAll('article')[0]!
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
    await expand('step-audience')
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
    expect(reply.getAttribute('data-author')).toBe('user')
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

    // The failure and its action are in the step's accordion, folded until unfolded (D-112).
    await expand('step-content')
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

  it("reopens an approved step from that step's own accordion after a confirmation", async () => {
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

    // The action is in the accordion of the approved step, which is folded until unfolded; the
    // open step has none, and neither does the end of the chat.
    expect(hasButton('Yeniden aç', accordion('step-audience')!)).toBe(false)
    await expand('step-audience')
    expect(hasButton('Yeniden aç', openStep()!)).toBe(false)
    expect(finalActions()).toBeNull()
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
    // It is the last entry of the chat and opens by itself: the confirmation needs attention.
    expect(chat().lastElementChild).toBe(finalActions())
    expect(finalTrigger().getAttribute('aria-expanded')).toBe('true')
    vi.mocked(proposal.confirmCampaign).mockResolvedValue(viewOf({ status: 'scheduled', steps }))

    await button('Onayı yeniden dene', chat()).trigger('click')
    await flushPromises()

    expect(proposal.confirmCampaign).toHaveBeenCalledWith('c1')
    expect(hasButton('Onayı yeniden dene', chat())).toBe(false)
    expect(finalActions()).toBeNull()
    expect(wrapper!.get('header [data-status]').attributes('data-status')).toBe('scheduled')
  })

  it('cancels with an optional reason after a confirmation', async () => {
    await open(viewOf({ steps: flowUpTo('audience') }))
    vi.mocked(proposal.cancelCampaign).mockResolvedValue(
      viewOf({ status: 'cancelled', steps: flowUpTo('audience') }),
    )

    await button('İptal et', chatBar()).trigger('click')
    await flushPromises()
    await new DOMWrapper(dialog()!.querySelector('textarea')!).setValue('  Yanlış indirim  ')
    await button('Kampanyayı iptal et', dialog()!).trigger('click')
    await flushPromises()

    expect(proposal.cancelCampaign).toHaveBeenCalledWith('c1', 'Yanlış indirim')
    expect(wrapper!.get('header [data-status]').attributes('data-status')).toBe('cancelled')
    expect(hasButton('İptal et', chatBar())).toBe(false)
  })

  it('cancels without a reason when none is typed', async () => {
    await open(viewOf({ steps: flowUpTo('audience') }))
    vi.mocked(proposal.cancelCampaign).mockResolvedValue(viewOf({ status: 'cancelled' }))

    await button('İptal et', chatBar()).trigger('click')
    await flushPromises()
    await button('Kampanyayı iptal et', dialog()!).trigger('click')
    await flushPromises()

    expect(proposal.cancelCampaign).toHaveBeenCalledWith('c1', undefined)
  })
})

describe('chat behaviour', () => {
  it('starts with the open step folded and unfolds it on a click (D-112)', async () => {
    const steps = flowUpTo('audience')
    steps[0]!.variants = [audienceDraft()]
    await open(viewOf({ steps }))

    const block = openStep()!
    expect(trigger('step-audience').getAttribute('aria-expanded')).toBe('false')
    // Folded: the events are not drawn, the proposal card still is.
    expect(block.textContent).not.toContain('Kitle adımı açıldı')
    expect(block.querySelector('article[data-slot="card"]')).not.toBeNull()

    await expand('step-audience')
    expect(trigger('step-audience').getAttribute('aria-expanded')).toBe('true')
    expect(block.textContent).toContain('Kitle adımı açıldı')

    await expand('step-audience')
    expect(trigger('step-audience').getAttribute('aria-expanded')).toBe('false')
  })

  it('lets the reader unfold the open step and an approved one, and keeps the choice', async () => {
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
    // Both start folded (D-112).
    expect(trigger('step-audience').getAttribute('aria-expanded')).toBe('false')
    expect(trigger('step-channel_plan').getAttribute('aria-expanded')).toBe('false')

    await expand('step-audience')
    await expand('step-channel_plan')
    expect(trigger('step-audience').getAttribute('aria-expanded')).toBe('true')
    expect(trigger('step-channel_plan').getAttribute('aria-expanded')).toBe('true')

    // A refetch (any event) does not undo what the reader chose.
    FakeEventSource.last.emit('snapshot', viewOf({ steps }))
    await flushPromises()
    expect(trigger('step-audience').getAttribute('aria-expanded')).toBe('true')
    expect(trigger('step-channel_plan').getAttribute('aria-expanded')).toBe('true')
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

    await new DOMWrapper(document.body.querySelector('header [aria-label="Menüyü aç"]')!).trigger(
      'click',
    )
    await flushPromises()
    expect(
      document.body.querySelector('[role="dialog"] [aria-label="Yeni kampanya"]'),
    ).not.toBeNull()
  })
})

/**
 * Elements under `root` (itself included) that look like a box of their own: a border, a rounded
 * corner, a shadow or a filled background. Controls, badges, icons and the frame of an email are
 * widgets, not boxes, and so are the cards when `skipCards` says the check is about what is around
 * them; their insides are not looked at.
 */
function boxes(root: Element, { skipCards = false } = {}) {
  const widgets = ['button', 'input', 'textarea', 'select', 'iframe', 'svg', '[data-slot="badge"]']
  if (skipCards) widgets.push('[data-slot="card"]')
  const skip = widgets.join(',')
  const isBox = (el: Element) =>
    [...el.classList].some(
      (c) =>
        c === 'border' ||
        c.startsWith('rounded') ||
        c.startsWith('shadow') ||
        (c.startsWith('bg-') && c !== 'bg-transparent'),
    )
  const found: Element[] = []
  const walk = (el: Element) => {
    if (el !== root && el.matches(skip)) return
    if (isBox(el)) found.push(el)
    for (const child of el.children) walk(child)
  }
  walk(root)
  return found
}

describe('plain chat (D-091)', () => {
  const question = {
    id: 'q1',
    campaign_id: 'c1',
    step_id: 'step-audience',
    kind: 'question' as const,
    body: 'Hangi **müşteriler**?',
    options: ['Sadece aktifler', 'Herkes'],
    created_at: '2026-10-30T10:00:03Z',
  }

  it('draws messages as plain lines: a label, the time and the text, with no box', async () => {
    const steps = flowUpTo('audience', { pending_question: question })
    steps[0]!.variants = [audienceDraft()]
    await open(
      viewOf({
        steps,
        notes: [{ id: 'n1', body: 'Hafta sonu', created_at: '2026-10-30T10:00:02Z' }],
        messages: [
          question,
          {
            id: 'm1',
            campaign_id: 'c1',
            step_id: 'step-audience',
            kind: 'summary',
            body: '**48 kişi**',
            created_at: '2026-10-30T10:00:04Z',
          },
          {
            id: 'a1',
            campaign_id: 'c1',
            step_id: 'step-audience',
            kind: 'answer',
            body: 'Herkes',
            reply_to: 'q1',
            created_at: '2026-10-30T10:00:05Z',
          },
        ],
      }),
    )

    const messages = [...chat().querySelectorAll(':scope > li[data-author]')]
    expect(messages.map((li) => li.getAttribute('data-kind'))).toEqual([
      'brief',
      'note',
      'question',
      'summary',
      'answer',
    ])
    for (const message of messages) {
      expect(message.tagName).toBe('LI')
      expect(boxes(message)).toEqual([])
      expect(message.querySelector('article, [data-slot="card"]')).toBeNull()
      // A role label and a time; the text is under them.
      expect(message.querySelector('p > span.font-medium')).not.toBeNull()
      expect(message.querySelector('time')).not.toBeNull()
    }
    // What the user wrote is right-aligned as a whole, label row and text; the agent's is not (D-101).
    for (const message of messages) {
      const user = message.getAttribute('data-author') === 'user'
      const header = message.querySelector('p')!
      expect(message.classList.contains('items-end')).toBe(user)
      expect(message.classList.contains('text-right')).toBe(user)
      expect(header.classList.contains('justify-end')).toBe(user)
      const body = message.querySelector('p.whitespace-pre-wrap')
      expect(body?.classList.contains('text-right') ?? user).toBe(user)
      expect(body?.classList.contains('max-w-[85%]') ?? user).toBe(user)
    }
    // The question keeps its options as buttons under the plain line.
    const asked = chat().querySelector('li[data-kind="question"]')!
    expect(asked.querySelector('strong')!.textContent).toBe('müşteriler')
    expect(hasButton('Sadece aktifler', asked)).toBe(true)
    expect(hasButton('Herkes', asked)).toBe(true)
  })

  it('draws the events of the campaign as plain lines too', async () => {
    await open(viewOf({ steps: flowUpTo('audience') }))
    FakeEventSource.last.emit('campaign_cancelled', {
      at: '2026-10-30T10:00:08Z',
      campaign_id: 'c1',
      reason: 'Yanlış indirim',
    })
    await flushPromises()

    const line = chat().querySelector('li[data-kind="campaignCancelled"]')!
    expect(line.textContent).toContain('Kampanya iptal edildi')
    expect(line.textContent).toContain('Yanlış indirim')
    expect(boxes(line)).toEqual([])
  })

  it('gives each step exactly one accordion, folded for the open step and the approved ones alike', async () => {
    const steps = flowUpTo('channel_plan')
    await open(viewOf({ steps }))

    const blocks = [...chat().querySelectorAll('li[data-kind="step"]')]
    expect(blocks.map((b) => b.getAttribute('data-step-id'))).toEqual([
      'step-audience',
      'step-channel_plan',
    ])
    for (const block of blocks) {
      expect(block.querySelectorAll('[data-slot="accordion"]')).toHaveLength(1)
      expect(block.querySelectorAll('[data-slot="accordion-item"]')).toHaveLength(1)
    }
    expect(trigger('step-audience').getAttribute('aria-expanded')).toBe('false')
    expect(trigger('step-channel_plan').getAttribute('aria-expanded')).toBe('false')
    // The events are in the accordion, not beside it.
    expect(chat().querySelectorAll(':scope > li[data-kind="stepOpened"]')).toHaveLength(0)
    await expand('step-channel_plan')
    expect(openStep()!.querySelector('[data-slot="accordion"]')!.textContent).toContain(
      'Kanal planı adımı açıldı',
    )
  })

  it('keeps the actions of a step in its accordion: why it failed and the way to try again', async () => {
    const steps = flowUpTo('content', {
      failed: true,
      failure_reason: 'Model geçerli çıktı vermedi',
    })
    await open(viewOf({ steps }))

    await expand('step-content')
    const inside = openStep()!.querySelector('[data-slot="accordion"]')!
    expect(inside.textContent).toContain('Üretim başarısız oldu.')
    expect(inside.textContent).toContain('Model geçerli çıktı vermedi')
    expect(hasButton('Yeniden üret', inside)).toBe(true)
    expect(boxes(openStep()!, { skipCards: true })).toEqual([])
  })

  it('writes the streaming text inside the step accordion, as a plain line', async () => {
    await open(viewOf({ steps: flowUpTo('content') }))
    await expand('step-content')

    FakeEventSource.last.emit('content_delta', {
      at: '2026-10-30T10:00:02Z',
      campaign_id: 'c1',
      step_id: 'step-content',
      slot: { language: 'tr', channel: 'sms' },
      attempt: 1,
      text: 'Merhaba',
    })
    await flushPromises()

    const line = openStep()!.querySelector('[data-slot="accordion"] li[data-kind="streaming"]')!
    expect(line.textContent).toContain('Türkçe · SMS yazılıyor')
    expect(line.textContent).toContain('Merhaba')
    expect(boxes(line)).toEqual([])
  })

  it('shows the drafts of the open step as cards outside its accordion', async () => {
    const steps = contentSteps()
    vi.mocked(proposal.previewVariant).mockResolvedValue(rendered('Your cart Alex', 'x'))
    await open(viewOf({ steps }))

    const block = openStep()!
    const cards = [...block.querySelectorAll('[data-slot="card"]')]
    expect(cards).toHaveLength(2)
    const accordionEl = block.querySelector('[data-slot="accordion"]')!
    for (const card of cards) {
      expect(card.tagName).toBe('ARTICLE')
      expect(accordionEl.contains(card)).toBe(false)
      expect(card.getAttribute('data-variant-status')).toBe('draft')
    }
    // The variant's own actions are part of its card.
    expect(hasButton('Onayla', cards[0]!)).toBe(true)
    expect(hasButton('Reddet', cards[0]!)).toBe(true)
  })

  it('shows only the approved variants of an approved step and lists the others in its accordion without cards', async () => {
    const steps = flowUpTo('schedule', { status: 'approved' })
    steps[2]!.status = 'approved'
    steps[2]!.variants = [
      variantOf({
        id: 'ok',
        step_id: 'step-content',
        kind: 'content',
        language: 'tr',
        channel: 'sms',
        status: 'approved',
        content: { text: 'Onaylanan metin', encoding: 'gsm7', segments: 1, rendered_length: 15 },
      }),
      variantOf({
        id: 'no',
        step_id: 'step-content',
        kind: 'content',
        language: 'tr',
        channel: 'sms',
        status: 'rejected',
        rejected_at: '2026-10-30T10:00:02Z',
        rejection_reason: 'Çok resmi',
        content: { text: 'Reddedilen metin', encoding: 'gsm7', segments: 1, rendered_length: 16 },
      }),
      variantOf({
        id: 'old',
        step_id: 'step-content',
        kind: 'content',
        language: 'en',
        channel: 'sms',
        status: 'superseded',
        content: { text: 'Eski metin', encoding: 'gsm7', segments: 1, rendered_length: 10 },
      }),
    ]
    await open(viewOf({ status: 'scheduled', steps }))

    const block = accordion('step-content')!
    const cards = block.querySelectorAll('[data-slot="card"]')
    expect(cards).toHaveLength(1)
    expect(cards[0]!.getAttribute('data-variant-status')).toBe('approved')
    expect(cards[0]!.textContent).toContain('Onaylanan metin')
    // The step is folded, so the other variants are not on screen.
    expect(block.textContent).not.toContain('Reddedilen metin')
    expect(block.textContent).not.toContain('Diğer varyantlar')

    await expand('step-content')
    expect(block.textContent).toContain('Diğer varyantlar')
    const listed = [...block.querySelectorAll('li[data-variant-status]')]
    expect(listed.map((li) => [li.getAttribute('data-variant-status'), words(li)])).toEqual([
      ['rejected', 'Türkçe · SMS Reddedildi'],
      ['superseded', 'İngilizce · SMS Geçersiz'],
    ])
    expect(block.querySelectorAll('[data-slot="card"]')).toHaveLength(1)
    expect(block.querySelector('article[data-variant-status="rejected"]')).toBeNull()
    expect(boxes(block, { skipCards: true })).toEqual([])
    // The rejected text is not shown again; the reason stays where the marketer wrote it.
    expect(block.textContent).not.toContain('Reddedilen metin')
  })

  it('has one layer of cards: no card in a card and no box of its own inside a card or around the events', async () => {
    vi.mocked(proposal.previewVariant).mockResolvedValue(rendered('Your cart Alex', 'x'))
    const steps = contentSteps()
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
        status: 'approved',
        content: planContent,
      }),
    ]
    await open(viewOf({ steps }))
    // An email under edit, with its form and the preview next to it.
    const email = openStep()!.querySelectorAll('article')[0]!
    await button('Düzenle', email).trigger('click')
    await flushPromises()
    vi.mocked(proposal.editVariant).mockRejectedValueOnce(
      ApiError.from(422, {
        code: 'invalid_content',
        title: 'Invalid content',
        detail: 'The content did not pass validation.',
        problems: ['email.cta_label must not be empty'],
      }),
    )
    await new DOMWrapper(email.querySelector('[name="cta_label"]')!).setValue('')
    await email.querySelector('form')!.dispatchEvent(new Event('submit', { cancelable: true }))
    await flushPromises()
    expect(email.querySelector('[role="alert"]')).not.toBeNull()

    const cards = [...chat().querySelectorAll('[data-slot="card"]')]
    expect(cards.length).toBeGreaterThanOrEqual(4)
    for (const card of cards) {
      // The card itself is the box; nothing inside it is another one.
      expect(card.querySelector('[data-slot="card"]')).toBeNull()
      expect(card.parentElement!.closest('[data-slot="card"]')).toBeNull()
      expect(boxes(card).filter((el) => el !== card)).toEqual([])
    }
    // The cards are variants, nothing else.
    expect(
      cards.every((c) => c.tagName === 'ARTICLE' && c.hasAttribute('data-variant-status')),
    ).toBe(true)
    // Around the cards: the accordions and their lines are plain.
    for (const block of chat().querySelectorAll('li[data-kind="step"]')) {
      expect(boxes(block, { skipCards: true })).toEqual([])
    }
  })

  it('puts the cancel button in the chat bar, next to send, while the campaign can be cancelled', async () => {
    await open(viewOf({ steps: flowUpTo('audience') }))

    expect(hasButton('İptal et', chat())).toBe(false)
    const cancel = button('İptal et', chatBar())
    const send = button('Not ekle', chatBar())
    expect(cancel.element.parentElement).toBe(send.element.parentElement)
  })

  it.each([
    ['scheduled', true],
    ['sending', true],
    ['completed', false],
    ['cancelled', false],
  ] as const)('offers cancel in the chat bar of a %s campaign: %s', async (status, offered) => {
    await open(viewOf({ status, steps: flowUpTo('audience', { status: 'approved' }) }))

    expect(hasButton('İptal et', chatBar())).toBe(offered)
    expect(hasButton('İptal et', chat())).toBe(false)
  })

  it('keeps the cancel button next to the answer box while the agent waits for an answer', async () => {
    await open(viewOf({ steps: flowUpTo('audience', { pending_question: question }) }))

    expect(hasButton('Cevapla', chatBar())).toBe(true)
    expect(hasButton('İptal et', chatBar())).toBe(true)
  })

  describe('accordion at the end of the chat', () => {
    it('is not there when nothing can be retried or restarted', async () => {
      await open(viewOf({ steps: flowUpTo('audience') }))
      expect(finalActions()).toBeNull()
    })

    it('is not there when only a reopen is possible; the reopen is in the step accordions', async () => {
      await open(viewOf({ steps: flowUpTo('content') }))

      expect(finalActions()).toBeNull()
      await expand('step-audience')
      await expand('step-channel_plan')
      expect(hasButton('Yeniden aç', accordion('step-audience')!)).toBe(true)
      expect(hasButton('Yeniden aç', accordion('step-channel_plan')!)).toBe(true)
      expect(hasButton('Yeniden aç', openStep()!)).toBe(false)
    })

    it('opens by itself with the restart of a start that produced no step', async () => {
      await open(viewOf({ steps: [] }))

      expect(finalTrigger().getAttribute('aria-expanded')).toBe('true')
      expect(finalActions()!.textContent).toContain('henüz adım oluşmadı')
      expect(hasButton('Yeniden başlat', finalActions()!)).toBe(true)
      expect(chat().lastElementChild).toBe(finalActions())
    })

    it('is not there once the campaign is no longer a draft', async () => {
      await open(
        viewOf({ status: 'scheduled', steps: flowUpTo('schedule', { status: 'approved' }) }),
      )
      expect(finalActions()).toBeNull()
    })
  })
})
