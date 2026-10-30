import { flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, NetworkError } from '@/api/errors'
import * as analytics from '@/api/analytics/service'
import * as proposal from '@/api/proposal/service'
import { resetToasts } from '@/features/campaigns/composables/useToasts'
import {
  FakeEventSource,
  audienceContent,
  mountApp,
  stepOf,
  variantOf,
  viewOf,
} from '@/__tests__/helpers'

vi.mock('@/api/proposal/service')
vi.mock('@/api/analytics/service')

let wrapper: Awaited<ReturnType<typeof mountApp>>['wrapper'] | undefined

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
  FakeEventSource.install()
  vi.mocked(proposal.getLlmMode).mockResolvedValue('anthropic')
  vi.mocked(proposal.listCampaigns).mockResolvedValue([])
  vi.mocked(analytics.getCampaignSummary).mockResolvedValue(null)
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  resetToasts()
  vi.useRealTimers()
  vi.resetAllMocks()
})

async function open(path = '/campaigns/c1') {
  const mounted = await mountApp(path)
  wrapper = mounted.wrapper
  return mounted
}

const kinds = () => wrapper!.findAll('li[data-kind]').map((li) => li.attributes('data-kind'))
const timeline = () => wrapper!.get('ol[aria-label="Olay zaman çizelgesi"]')

/** Lets the debounced refetch that events schedule run. */
const settle = async () => {
  await vi.advanceTimersByTimeAsync(300)
  await flushPromises()
}

const at = (second: number) => `2026-10-30T10:00:${String(second).padStart(2, '0')}.000Z`

describe('timeline', () => {
  it('renders the brief, then the system cards of the view and the stream in time order', async () => {
    const audience = stepOf('audience', { id: 's1', created_at: at(2) })
    audience.variants = [
      variantOf({
        id: 'v1',
        step_id: 's1',
        kind: 'audience',
        created_at: at(4),
        content: audienceContent,
      }),
    ]
    vi.mocked(proposal.getCampaign).mockResolvedValue(
      viewOf({ created_at: at(0), steps: [audience] }),
    )
    await open()
    const es = FakeEventSource.last

    es.emit('screen_result', { at: at(1), campaign_id: 'c1', verdict: 'allow', reason: '' })
    es.emit('generation_started', {
      at: at(3),
      campaign_id: 'c1',
      step_id: 's1',
      kind: 'audience',
      slots: [{}],
    })
    await flushPromises()

    expect(kinds()).toEqual([
      'brief',
      'screen',
      'stepOpened',
      'generationStarted',
      'variantCreated',
    ])
    const text = timeline().text()
    expect(text.indexOf('İstanbul için %30 indirim hatırlatması')).toBeLessThan(
      text.indexOf('Brief ön elemeden geçti'),
    )
    expect(text).toContain('Kitle adımı açıldı')
    expect(text).toContain('Kitle için üretim başladı')
    expect(text).toContain('Kitle varyantı hazır')
  })

  it('shows context notes and rejection reasons as the marketer’s own messages', async () => {
    const content = stepOf('content', { id: 's3', created_at: at(2) })
    content.variants = [
      variantOf({
        id: 'v9',
        step_id: 's3',
        kind: 'content',
        language: 'tr',
        channel: 'sms',
        status: 'rejected',
        created_at: at(3),
        rejected_at: at(6),
        rejection_reason: 'Çok resmi,\ndaha samimi olsun.',
      }),
    ]
    vi.mocked(proposal.getCampaign).mockResolvedValue(
      viewOf({
        created_at: at(0),
        notes: [
          {
            id: 'n1',
            body: 'Hafta sonu kampanyası\nolduğunu belirt.',
            created_at: at(5),
            step_id: 's3',
          },
        ],
        steps: [content],
      }),
    )
    await open()

    const user = wrapper!
      .findAll('li[data-kind]')
      .filter((li) => li.classes().includes('justify-end'))
    expect(user.map((li) => li.attributes('data-kind'))).toEqual(['brief', 'note', 'rejection'])
    expect(user[1]!.text()).toContain('Hafta sonu kampanyası')
    expect(user[2]!.text()).toContain('Red gerekçesi · Metin · Türkçe · SMS')
    expect(user[2]!.text()).toContain('daha samimi olsun.')
    // Line breaks are kept by CSS, not by markup: the text stays plain.
    expect(user[2]!.find('.whitespace-pre-wrap').exists()).toBe(true)
    expect(user[2]!.find('br').exists()).toBe(false)
  })

  it('draws brief text literally, never as markup', async () => {
    vi.mocked(proposal.getCampaign).mockResolvedValue(
      viewOf({ brief: '<img src=x onerror=alert(1)> **kalın** [link](http://x)' }),
    )
    await open()

    const card = wrapper!.get('li[data-kind="brief"]')
    expect(card.find('img').exists()).toBe(false)
    expect(card.text()).toContain('<img src=x onerror=alert(1)> **kalın** [link](http://x)')
  })

  it('adds cards for events only the stream knows: failure with its reason, cancel, sending, confirmation', async () => {
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf({ created_at: at(0) }))
    await open()
    const es = FakeEventSource.last

    es.emit('generation_failed', {
      at: at(3),
      campaign_id: 'c1',
      step_id: 's2',
      kind: 'content',
      slot: { language: 'tr', channel: 'sms' },
      reason: 'İçerik 3 denemede doğrulanamadı',
    })
    es.emit('step_superseded', {
      at: at(4),
      campaign_id: 'c1',
      step_id: 's2',
      kind: 'content',
      version: 1,
    })
    es.emit('campaign_confirmed', {
      at: at(5),
      campaign_id: 'c1',
      recipient_batches: 1,
      recipient_counts: { total: 46, email: 33, sms: 13 },
    })
    es.emit('campaign_status', { at: at(6), campaign_id: 'c1', status: 'sending' })
    es.emit('campaign_status', { at: at(7), campaign_id: 'c1', status: 'completed' })
    es.emit('campaign_cancelled', { at: at(8), campaign_id: 'c1', reason: 'Yanlış indirim' })
    await flushPromises()

    expect(kinds()).toEqual([
      'brief',
      'generationFailed',
      'stepSuperseded',
      'campaignConfirmed',
      'campaignStatus',
      'campaignStatus',
      'campaignCancelled',
    ])
    const text = timeline().text()
    expect(text).toContain('Metin · Türkçe · SMS üretilemedi')
    expect(text).toContain('İçerik 3 denemede doğrulanamadı')
    expect(text).toContain('Metin adımı geçersiz kılındı')
    expect(text).toContain('46 alıcı (33 e-posta, 13 SMS)')
    expect(text).toContain('Gönderim başladı')
    expect(text).toContain('Gönderim tamamlandı')
    expect(text).toContain('Kampanya iptal edildi')
    expect(text).toContain('Yanlış indirim')
  })

  it('shows a blocked screening result with its reason', async () => {
    vi.mocked(proposal.getCampaign).mockResolvedValue(
      viewOf({ status: 'cancelled', created_at: at(0) }),
    )
    await open()

    FakeEventSource.last.emit('screen_result', {
      at: at(1),
      campaign_id: 'c1',
      verdict: 'block',
      reason: 'Yasa dışı içerik',
    })
    await flushPromises()

    expect(timeline().text()).toContain('Brief ön elemede engellendi')
    expect(timeline().text()).toContain('Yasa dışı içerik')
  })

  it('does not duplicate a card that the stream and the view both know', async () => {
    const audience = stepOf('audience', { id: 's1', created_at: at(2) })
    vi.mocked(proposal.getCampaign).mockResolvedValue(
      viewOf({ created_at: at(0), steps: [audience] }),
    )
    await open()

    FakeEventSource.last.emit('step_opened', {
      at: at(2),
      campaign_id: 'c1',
      step_id: 's1',
      kind: 'audience',
      version: 1,
    })
    await settle()

    expect(kinds().filter((k) => k === 'stepOpened')).toHaveLength(1)
  })

  it('streams content per slot, starts over on a reset and drops the card when the variant arrives', async () => {
    const content = stepOf('content', { id: 's3' })
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf({ steps: [content] }))
    await open()
    const es = FakeEventSource.last
    const sms = { language: 'tr', channel: 'sms' }
    const email = { language: 'tr', channel: 'email' }

    es.emit('content_delta', {
      at: at(1),
      campaign_id: 'c1',
      step_id: 's3',
      slot: sms,
      attempt: 1,
      text: '{"text":"Merh',
    })
    es.emit('content_delta', {
      at: at(1),
      campaign_id: 'c1',
      step_id: 's3',
      slot: sms,
      attempt: 1,
      text: 'aba',
    })
    es.emit('content_delta', {
      at: at(1),
      campaign_id: 'c1',
      step_id: 's3',
      slot: email,
      attempt: 1,
      text: 'Konu',
    })
    await flushPromises()

    const cards = () => wrapper!.findAll('li[data-kind="streaming"]')
    expect(cards()).toHaveLength(2)
    expect(cards()[0]!.text()).toContain('Türkçe · SMS yazılıyor')
    expect(cards()[0]!.text()).toContain('{"text":"Merhaba')
    expect(cards()[1]!.text()).toContain('Konu')

    es.emit('content_reset', { at: at(2), campaign_id: 'c1', step_id: 's3', slot: sms, attempt: 2 })
    es.emit('content_delta', {
      at: at(2),
      campaign_id: 'c1',
      step_id: 's3',
      slot: sms,
      attempt: 2,
      text: 'Yeni',
    })
    await flushPromises()
    expect(cards()[0]!.text()).toContain('Yeni')
    expect(cards()[0]!.text()).not.toContain('Merhaba')
    expect(cards()[0]!.text()).toContain('2. deneme')

    es.emit('variant_created', {
      at: at(3),
      campaign_id: 'c1',
      step_id: 's3',
      variant_id: 'v1',
      kind: 'content',
      slot: sms,
      content: {},
    })
    await flushPromises()
    expect(cards()).toHaveLength(1)
    expect(cards()[0]!.text()).toContain('E-posta')

    es.emit('generation_failed', {
      at: at(4),
      campaign_id: 'c1',
      step_id: 's3',
      kind: 'content',
      slot: email,
      reason: 'x',
    })
    await flushPromises()
    expect(cards()).toHaveLength(0)
  })

  it('refetches the view once for a burst of events and takes the snapshot of a reconnect as is', async () => {
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf({ status: 'drafting' }))
    await open()
    const es = FakeEventSource.last
    expect(proposal.getCampaign).toHaveBeenCalledTimes(1)

    for (const id of ['a', 'b', 'c']) {
      es.emit('variant_created', {
        at: at(1),
        campaign_id: 'c1',
        step_id: 's',
        variant_id: id,
        kind: 'content',
        slot: {},
        content: {},
      })
    }
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf({ status: 'scheduled' }))
    await settle()
    expect(proposal.getCampaign).toHaveBeenCalledTimes(2)
    expect(wrapper!.get('header [data-status]').attributes('data-status')).toBe('scheduled')

    es.emit('snapshot', viewOf({ status: 'sending' }))
    await flushPromises()
    expect(proposal.getCampaign).toHaveBeenCalledTimes(2)
    expect(wrapper!.get('header [data-status]').attributes('data-status')).toBe('sending')
  })

  it('reflects the status of the open campaign in the list', async () => {
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf({ id: 'c1', status: 'drafting' }))
    await open()

    expect(wrapper!.get('aside li').text()).toContain('Taslak')
    FakeEventSource.last.emit('snapshot', viewOf({ id: 'c1', status: 'cancelled' }))
    await flushPromises()

    expect(wrapper!.get('aside li').text()).toContain('İptal edildi')
  })
})

describe('event stream', () => {
  it('keeps one connection, closes it on a route change and opens the next campaign’s', async () => {
    vi.mocked(proposal.getCampaign).mockImplementation(async (id) => viewOf({ id }))
    const { router } = await open('/campaigns/c1')
    expect(FakeEventSource.open.map((es) => es.url)).toEqual(['/api/proposal/campaigns/c1/events'])

    await router.push('/campaigns/c2')
    await flushPromises()
    expect(FakeEventSource.instances[0]!.closed).toBe(true)
    expect(FakeEventSource.open.map((es) => es.url)).toEqual(['/api/proposal/campaigns/c2/events'])

    await router.push('/')
    await flushPromises()
    expect(FakeEventSource.open).toHaveLength(0)
  })

  it('forgets the previous campaign’s cards when another one opens', async () => {
    vi.mocked(proposal.getCampaign).mockImplementation(async (id) =>
      viewOf({ id, brief: `Brief of ${id}` }),
    )
    const { router } = await open('/campaigns/c1')
    FakeEventSource.last.emit('campaign_cancelled', {
      at: at(8),
      campaign_id: 'c1',
      reason: 'eski',
    })
    await flushPromises()
    expect(timeline().text()).toContain('Kampanya iptal edildi')

    await router.push('/campaigns/c2')
    await flushPromises()

    expect(timeline().text()).toContain('Brief of c2')
    expect(timeline().text()).not.toContain('Kampanya iptal edildi')
  })

  it('says so when the live connection drops', async () => {
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf())
    await open()

    FakeEventSource.last.onerror?.()
    await flushPromises()

    expect(wrapper!.text()).toContain('yeniden bağlanılıyor')
  })
})

describe('load states', () => {
  it('says the campaign was not found on a 404', async () => {
    vi.mocked(proposal.getCampaign).mockRejectedValue(
      ApiError.from(404, { code: 'not_found', title: 'Not found' }),
    )
    await open('/campaigns/gone')

    expect(wrapper!.get('[role="alert"]').text()).toContain('Bu kampanya bulunamadı.')
  })

  it('shows the retry hint on a network failure and loads on retry', async () => {
    vi.mocked(proposal.getCampaign).mockRejectedValueOnce(new NetworkError(new TypeError('x')))
    await open()
    expect(wrapper!.get('[role="alert"]').text()).toContain('Sunucuya ulaşılamadı')

    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf())
    await wrapper!
      .findAll('button')
      .find((b) => b.text() === 'Yeniden dene')!
      .trigger('click')
    await flushPromises()

    expect(kinds()).toContain('brief')
  })
})

describe('note box', () => {
  it('posts a context note on a drafting campaign and clears the box', async () => {
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf({ status: 'drafting' }))
    vi.mocked(proposal.addContext).mockResolvedValue({ id: 'n1', body: 'Not', created_at: at(1) })
    await open()

    const box = wrapper!.get('textarea')
    await box.setValue('Hafta sonu olduğunu belirt')
    await wrapper!.get('form').trigger('submit')
    await flushPromises()

    expect(proposal.addContext).toHaveBeenCalledWith('c1', 'Hafta sonu olduğunu belirt')
    expect((box.element as HTMLTextAreaElement).value).toBe('')
  })

  it('is disabled once the campaign is no longer a draft', async () => {
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf({ status: 'completed' }))
    await open()

    expect(wrapper!.get('textarea').attributes('disabled')).toBeDefined()
    expect(wrapper!.text()).toContain('Bu kampanyaya artık bağlam eklenemez.')
  })

  it('stops at 2000 characters', async () => {
    vi.mocked(proposal.getCampaign).mockResolvedValue(viewOf({ status: 'drafting' }))
    await open()

    await wrapper!.get('textarea').setValue('a'.repeat(2001))
    await wrapper!.get('form').trigger('submit')

    expect(proposal.addContext).not.toHaveBeenCalled()
    expect(wrapper!.text()).toContain('2.001/2.000')
  })
})
