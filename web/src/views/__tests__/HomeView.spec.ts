import { flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, NetworkError } from '@/api/errors'
import * as analytics from '@/api/analytics/service'
import * as proposal from '@/api/proposal/service'
import { resetToasts } from '@/features/campaigns/composables/useToasts'
import { FakeEventSource, infoOf, mountApp, viewOf } from '@/__tests__/helpers'

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
  resetToasts()
  vi.resetAllMocks()
})

async function open() {
  const mounted = await mountApp('/')
  wrapper = mounted.wrapper
  return mounted
}

const submitButton = (w: NonNullable<typeof wrapper>) =>
  w.findAll('button').find((b) => b.text() === 'Kampanyayı başlat')!

describe('HomeView', () => {
  it('posts the brief, adds the campaign to the list and opens it', async () => {
    const created = infoOf({ id: 'new-1', brief: 'Yeni bir brief' })
    vi.mocked(proposal.createCampaign).mockResolvedValue(created)
    vi.mocked(proposal.getCampaign).mockResolvedValue(
      viewOf({ id: 'new-1', brief: 'Yeni bir brief' }),
    )
    const { wrapper, router } = await open()

    await wrapper.get('textarea').setValue('  Yeni bir brief  ')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(proposal.createCampaign).toHaveBeenCalledTimes(1)
    expect(proposal.createCampaign).toHaveBeenCalledWith('Yeni bir brief')
    expect(router.currentRoute.value.fullPath).toBe('/campaigns/new-1')
    // The list shows it without a reload, and the campaign page opened the event stream.
    expect(wrapper.get('aside').text()).toContain('Yeni bir brief')
    expect(FakeEventSource.last.url).toBe('/api/proposal/campaigns/new-1/events')
  })

  it('keeps the send button off for an empty or over-long brief and counts characters', async () => {
    const { wrapper } = await open()

    expect(submitButton(wrapper).attributes('disabled')).toBeDefined()

    await wrapper.get('textarea').setValue('ğ'.repeat(2000))
    expect(wrapper.text()).toContain('2.000/2.000')
    expect(submitButton(wrapper).attributes('disabled')).toBeUndefined()

    await wrapper.get('textarea').setValue('ğ'.repeat(2001))
    expect(wrapper.text()).toContain('2.001/2.000')
    expect(submitButton(wrapper).attributes('disabled')).toBeDefined()
    expect(proposal.createCampaign).not.toHaveBeenCalled()
  })

  it('shows the validation problems of a 422 next to the box and keeps the text', async () => {
    vi.mocked(proposal.createCampaign).mockRejectedValue(
      ApiError.from(422, {
        code: 'invalid_input',
        title: 'Invalid input',
        detail: 'The brief is not valid.',
        problems: ['brief: must not be empty'],
      }),
    )
    const { wrapper, router } = await open()

    await wrapper.get('textarea').setValue('x')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(wrapper.get('[role="alert"]').text()).toContain('The brief is not valid.')
    expect(wrapper.get('[role="alert"]').text()).toContain('brief: must not be empty')
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('x')
    expect(router.currentRoute.value.fullPath).toBe('/')
  })

  it('tells the user to retry when the network fails', async () => {
    vi.mocked(proposal.createCampaign).mockRejectedValue(new NetworkError(new TypeError('x')))
    const { wrapper } = await open()

    await wrapper.get('textarea').setValue('x')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(document.body.textContent).toContain('Sunucuya ulaşılamadı')
    expect(document.body.textContent).toContain('yeniden deneyin')
  })
})
