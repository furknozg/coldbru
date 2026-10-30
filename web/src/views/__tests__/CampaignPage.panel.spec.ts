import { DOMWrapper, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError } from '@/api/errors'
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

const panel = () => document.body.querySelector<HTMLElement>('[data-testid="step-panel"]')!

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

describe('step panel contents', () => {
  it('lists the four steps in flow order with their status and shows the audience of the open step', async () => {
    const steps = flowUpTo('audience')
    steps[0]!.variants = [audienceDraft()]
    await open(viewOf({ steps }))

    const nav = panel().querySelector('nav')!
    expect([...nav.querySelectorAll('li')].map((li) => words(li))).toEqual([
      '1. Kitle Açık',
      '2. Kanal planı Bekliyor',
      '3. Metin Bekliyor',
      '4. Zamanlama Bekliyor',
    ])

    const text = panel().textContent!
    expect(panel().querySelector('pre')!.textContent).toBe(audienceContent.dsl)
    expect(panel().querySelector('pre')!.className).toContain('font-mono')
    expect(text).toContain('48')
    const rows = [...panel().querySelectorAll('table')[0]!.querySelectorAll('tr')].map((tr) =>
      words(tr),
    )
    expect(rows).toEqual(['İkisi de 22', 'Yalnız e-posta 13', 'Yalnız SMS 13', 'Ulaşılamaz 3'])
    // The by_locale breakdown: language, time zone and the four coverage counts.
    const breakdown = panel().querySelectorAll('table')[1]!.textContent!
    expect(breakdown).toContain('Türkçe')
    expect(breakdown).toContain('Europe/Istanbul')
    expect(breakdown).toContain('İngilizce')
    expect(breakdown).toContain('Europe/London')
  })

  it('shows the channel plan with mode, primary channel, rationale and recipients per channel and language', async () => {
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

    const text = words(panel())
    expect(text).toContain('Mod Birincil + yedek')
    expect(text).toContain('Birincil kanal E-posta')
    expect(text).toContain('E-posta daha çok kişiye ulaşıyor.')
    expect(text).toContain('Türkçe 29 13')
    expect(text).toContain('İngilizce 6 0')
    expect(text).toContain('Toplam 48 35 13')
  })

  it('shows SMS text with encoding, segments and cost, and email blocks as text, read only', async () => {
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
        content: {
          subject: 'Your cart',
          preheader: 'Pre',
          headline: 'Head',
          paragraphs: ['One', 'Two'],
          cta_label: 'Go',
          estimated_cost_usd: '0.0060',
        },
      }),
    ]
    await open(viewOf({ steps }))

    const cards = [...panel().querySelectorAll('article')]
    expect(cards[0]!.textContent).toContain(
      'Merhaba {{first_name}}, %30 indirim <b>seni</b> bekliyor',
    )
    expect(cards[0]!.querySelector('b')).toBeNull()
    expect(cards[0]!.textContent).toContain('UCS-2')
    expect(words(cards[0]!)).toContain('Segment 2')
    expect(cards[0]!.textContent).toContain('$1,04')
    expect(cards[1]!.textContent).toContain('Your cart')
    expect(cards[1]!.textContent).toContain('One')
    expect(cards[1]!.textContent).toContain('Two')
    expect(cards[1]!.textContent).toContain('$0,006')
    // Approve and reject are there; editing a text belongs to T-036.
    expect(hasButton('Onayla', cards[0]!)).toBe(true)
    expect(hasButton('Düzenle', cards[0]!)).toBe(false)
    expect(hasButton('Düzenle', cards[1]!)).toBe(false)
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

    const [tr, en] = [...panel().querySelectorAll('article')]
    expect(tr!.textContent).toContain('Türkçe · Europe/Istanbul')
    expect(tr!.textContent).toContain('09:30')
    expect(tr!.textContent).toContain('Pazar sabahı')
    expect(en!.textContent).toContain('Hemen gönder')
    expect(hasButton('Düzenle', tr!)).toBe(false)
  })

  it('shows the cost summary from the view', async () => {
    const steps = flowUpTo('content')
    await open(
      viewOf({ steps, cost: { sms_usd: '1.0400', email_usd: '0.0330', total_usd: '1.0730' } }),
    )

    const text = words(panel())
    expect(text).toContain('E-posta $0,033')
    expect(text).toContain('SMS $1,04')
    expect(text).toContain('Toplam $1,073')
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

    await button('Onayla', panel()).trigger('click')
    await flushPromises()

    expect(proposal.approveVariant).toHaveBeenCalledWith('va')
    expect(panel().textContent).toContain('Kanal planı')
    expect(panel().textContent).toContain('E-posta daha çok kişiye ulaşıyor.')
    expect(panel().querySelector('[aria-current="step"]')!.textContent).toContain('Kanal planı')
  })

  it('needs a reason to reject, sends it trimmed and closes the dialog', async () => {
    await openAudience()
    vi.mocked(proposal.rejectVariant).mockResolvedValue(viewOf({ steps: flowUpTo('audience') }))

    await button('Reddet', panel()).trigger('click')
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
    await button('Reddet', panel()).trigger('click')
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

    await button('Onayla', panel()).trigger('click')
    await flushPromises()

    const toast = document.body.querySelector('[role="alert"]')!
    expect(toast.textContent).toContain('Adım artık açık değil.')
    expect(vi.mocked(proposal.getCampaign).mock.calls.length).toBe(before + 1)
  })

  it('offers no approve or reject once the campaign is not a draft', async () => {
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

    expect(hasButton('Onayla', panel())).toBe(false)
    expect(hasButton('Reddet', panel())).toBe(false)
    expect(hasButton('Yeniden aç', panel())).toBe(false)
    expect(hasButton('İptal et', panel())).toBe(true)
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

    await button('Düzenle', panel()).trigger('click')
    const area = new DOMWrapper(panel().querySelector('textarea')!)
    expect((area.element as HTMLTextAreaElement).value).toBe(audienceContent.dsl)
    await area.setValue('citty === "İstanbul"')
    await panel()
      .querySelector('form')!
      .dispatchEvent(new Event('submit', { cancelable: true }))
    await flushPromises()

    expect(proposal.editVariant).toHaveBeenCalledWith('va', { dsl: 'citty === "İstanbul"' })
    const alert = panel().querySelector('[role="alert"]')!
    expect(alert.textContent).toContain('The audience rule is not valid.')
    expect(alert.textContent).toContain('unknown field "citty"')
    expect(alert.textContent).toContain('line 1: expected an expression')
    // A 422 belongs to the form: no toast, and the form stays open with the text.
    expect(document.body.querySelectorAll('[role="alert"]')).toHaveLength(1)
    expect((panel().querySelector('textarea') as HTMLTextAreaElement).value).toBe(
      'citty === "İstanbul"',
    )

    // Fixed: the edit goes through, the view is fetched again and the form closes.
    vi.mocked(proposal.editVariant).mockResolvedValueOnce(variantOf())
    const before = vi.mocked(proposal.getCampaign).mock.calls.length
    await area.setValue('city === "İzmir"')
    await panel()
      .querySelector('form')!
      .dispatchEvent(new Event('submit', { cancelable: true }))
    await flushPromises()

    expect(proposal.editVariant).toHaveBeenLastCalledWith('va', { dsl: 'city === "İzmir"' })
    expect(vi.mocked(proposal.getCampaign).mock.calls.length).toBe(before + 1)
    expect(panel().querySelector('form')).toBeNull()
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

    await button('Düzenle', panel()).trigger('click')
    const [mode, primary] = [...panel().querySelectorAll('form select')] as HTMLSelectElement[]
    expect([...mode!.options].map((o) => o.text)).toEqual([
      'Tek kanal',
      'Birincil + yedek',
      'İki kanal birden',
    ])
    expect(mode!.value).toBe('primary_fallback')
    expect(primary!.value).toBe('email')
    await new DOMWrapper(mode!).setValue('single')
    await new DOMWrapper(primary!).setValue('sms')
    await new DOMWrapper(panel().querySelector('form textarea')!).setValue('  Yalnız SMS  ')
    await panel()
      .querySelector('form')!
      .dispatchEvent(new Event('submit', { cancelable: true }))
    await flushPromises()

    expect(proposal.editVariant).toHaveBeenCalledWith('vp', {
      mode: 'single',
      primary: 'sms',
      rationale: 'Yalnız SMS',
    })
  })

  it('can leave the edit form without sending anything', async () => {
    const steps = flowUpTo('audience')
    steps[0]!.variants = [audienceDraft()]
    await open(viewOf({ steps }))

    await button('Düzenle', panel()).trigger('click')
    await button('Vazgeç', panel()).trigger('click')

    expect(panel().querySelector('form')).toBeNull()
    expect(proposal.editVariant).not.toHaveBeenCalled()
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

    expect(panel().textContent).toContain('Üretim başarısız oldu.')
    expect(panel().textContent).toContain('Model 3 denemede geçerli çıktı vermedi')
    await button('Yeniden üret', panel()).trigger('click')
    await flushPromises()

    expect(proposal.regenerateStep).toHaveBeenCalledWith('step-content')
    expect(panel().textContent).toContain('Üretiliyor…')
    expect(panel().textContent).not.toContain('Yeniden üret')
  })

  it('restarts a drafting campaign that has no steps', async () => {
    await open(viewOf({ steps: [] }))
    vi.mocked(proposal.regenerateCampaign).mockResolvedValue(
      viewOf({ steps: flowUpTo('audience') }),
    )

    expect(panel().textContent).toContain('Henüz adım yok.')
    await button('Yeniden başlat', panel()).trigger('click')
    await flushPromises()

    expect(proposal.regenerateCampaign).toHaveBeenCalledWith('c1')
    expect(hasButton('Yeniden başlat', panel())).toBe(false)
  })

  it('reopens an approved step after a confirmation', async () => {
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

    await button('1. Kitle Onaylandı', panel()).trigger('click')
    expect(panel().textContent).toContain('Onaylandı')
    await button('Yeniden aç', panel()).trigger('click')
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

    await button('Onayı yeniden dene', panel()).trigger('click')
    await flushPromises()

    expect(proposal.confirmCampaign).toHaveBeenCalledWith('c1')
    expect(hasButton('Onayı yeniden dene', panel())).toBe(false)
    expect(wrapper!.get('header [data-status]').attributes('data-status')).toBe('scheduled')
  })

  it('cancels with an optional reason after a confirmation', async () => {
    await open(viewOf({ steps: flowUpTo('audience') }))
    vi.mocked(proposal.cancelCampaign).mockResolvedValue(
      viewOf({ status: 'cancelled', steps: flowUpTo('audience') }),
    )

    await button('İptal et', panel()).trigger('click')
    await flushPromises()
    await new DOMWrapper(dialog()!.querySelector('textarea')!).setValue('  Yanlış indirim  ')
    await button('Kampanyayı iptal et', dialog()!).trigger('click')
    await flushPromises()

    expect(proposal.cancelCampaign).toHaveBeenCalledWith('c1', 'Yanlış indirim')
    expect(wrapper!.get('header [data-status]').attributes('data-status')).toBe('cancelled')
    expect(hasButton('İptal et', panel())).toBe(false)
  })

  it('cancels without a reason when none is typed', async () => {
    await open(viewOf({ steps: flowUpTo('audience') }))
    vi.mocked(proposal.cancelCampaign).mockResolvedValue(viewOf({ status: 'cancelled' }))

    await button('İptal et', panel()).trigger('click')
    await flushPromises()
    await button('Kampanyayı iptal et', dialog()!).trigger('click')
    await flushPromises()

    expect(proposal.cancelCampaign).toHaveBeenCalledWith('c1', undefined)
  })
})

describe('panel behaviour', () => {
  it('cannot be closed while a step is open, only collapsed', async () => {
    const steps = flowUpTo('audience')
    steps[0]!.variants = [audienceDraft()]
    await open(viewOf({ steps }))

    expect(hasButton('', panel())).toBe(true)
    expect(panel().querySelector('[aria-label="Paneli kapat"]')).toBeNull()
    const collapse = panel().querySelector<HTMLElement>('[aria-label="Paneli daralt"]')!
    collapse.click()
    await flushPromises()

    expect(document.body.querySelector('[data-testid="step-panel"]')).toBeNull()
    expect(document.body.querySelector('[aria-label="Paneli genişlet"]')).not.toBeNull()

    document.body.querySelector<HTMLElement>('[aria-label="Paneli genişlet"]')!.click()
    await flushPromises()
    expect(document.body.querySelector('[data-testid="step-panel"]')).not.toBeNull()
  })

  it('can be closed when nothing waits for a decision, and reopened from the header', async () => {
    const steps = flowUpTo('schedule', { status: 'approved' })
    await open(viewOf({ status: 'scheduled', steps }))

    expect(panel().querySelector('[aria-label="Paneli daralt"]')).toBeNull()
    panel().querySelector<HTMLElement>('[aria-label="Paneli kapat"]')!.click()
    await flushPromises()
    expect(document.body.querySelector('[data-testid="step-panel"]')).toBeNull()

    document.body.querySelector<HTMLElement>('header [aria-label="Adım panelini aç"]')!.click()
    await flushPromises()
    expect(document.body.querySelector('[data-testid="step-panel"]')).not.toBeNull()
  })

  it('brings a closed panel back when a new step opens', async () => {
    const approved = flowUpTo('audience', { status: 'approved' })
    await open(viewOf({ status: 'scheduled', steps: approved }))
    panel().querySelector<HTMLElement>('[aria-label="Paneli kapat"]')!.click()
    await flushPromises()
    expect(document.body.querySelector('[data-testid="step-panel"]')).toBeNull()

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

    expect(panel()).not.toBeNull()
    expect(panel().textContent).toContain('Kanal planı')
  })

  it('focuses a step when its card in the timeline is clicked', async () => {
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
    expect(panel().querySelector('[aria-current="step"]')!.textContent).toContain('Kanal planı')

    const card = wrapper!
      .findAll('li[data-kind="stepOpened"] button')
      .find((b) => b.text().includes('Kitle adımı açıldı'))!
    await card.trigger('click')

    expect(panel().querySelector('[aria-current="step"]')!.textContent).toContain('Kitle')
    expect(panel().querySelector('pre')!.textContent).toBe(audienceContent.dsl)
  })
})

describe('narrow screens', () => {
  it('moves the list and the step panel into drawers', async () => {
    setViewportWide(false)
    const steps = flowUpTo('audience')
    steps[0]!.variants = [audienceDraft()]
    await open(viewOf({ steps }))

    expect(wrapper!.find('aside').exists()).toBe(false)
    expect(document.body.querySelector('[data-testid="step-panel"]')).toBeNull()

    await new DOMWrapper(
      document.body.querySelector('header [aria-label="Adım panelini aç"]')!,
    ).trigger('click')
    await flushPromises()
    expect(panel().textContent).toContain('Kitle')
    // The drawer collapses, it does not discard the open step.
    expect(panel().querySelector('[aria-label="Paneli kapat"]')).toBeNull()
    panel().querySelector<HTMLElement>('[aria-label="Paneli daralt"]')!.click()
    await flushPromises()
    expect(document.body.querySelector('[data-testid="step-panel"]')).toBeNull()

    await new DOMWrapper(
      document.body.querySelector('header [aria-label="Kampanya listesini aç"]')!,
    ).trigger('click')
    await flushPromises()
    expect(document.body.textContent).toContain('Yeni kampanya')
  })
})
