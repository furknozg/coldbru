import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import { variantOf } from '@/__tests__/helpers'
import { createAppI18n } from '@/i18n'

import PlanBody from '../PlanBody.vue'

const content = {
  mode: 'primary_fallback',
  primary: 'email',
  rationale: 'E-posta ucuz; ulaşılamayanlara SMS.',
  recipients: { email: 36, sms: 13, total: 49 },
  by_language: [
    { language: 'tr', email: 32, sms: 11 },
    { language: 'en', email: 4, sms: 2 },
  ],
}

/** The pie is a lazy chunk (D-092): the body is mounted once it has loaded. */
async function mountBody(c: Record<string, unknown> = content) {
  const wrapper = mount(PlanBody, {
    props: { variant: variantOf({ kind: 'channel_plan', content: c }) },
    global: { plugins: [createAppI18n()] },
  })
  await vi.dynamicImportSettled()
  await flushPromises()
  return wrapper
}

describe('PlanBody', () => {
  it('keeps the mode, the primary channel and the rationale', async () => {
    const wrapper = await mountBody()
    expect(wrapper.text()).toContain('Birincil + yedek')
    expect(wrapper.text()).toContain('Birincil kanal')
    expect(wrapper.text()).toContain('E-posta ucuz')
  })

  it('charts the recipients per channel in one pie and has no table', async () => {
    const wrapper = await mountBody()
    const figures = wrapper.findAll('figure')
    expect(figures).toHaveLength(1)
    expect(figures[0]!.find('figcaption').text()).toBe('Kanal başına alıcılar')
    const rows = figures[0]!.findAll('li').map((li) => li.findAll('span').map((s) => s.text()))
    expect(rows).toEqual([
      ['E-posta', '36', '%73,5'],
      ['SMS', '13', '%26,5'],
    ])
    expect(wrapper.find('table').exists()).toBe(false)
  })

  it('gives a channel the same color in the pie as in the audience coverage', async () => {
    const wrapper = await mountBody()
    const swatches = wrapper.findAll('li > svg circle')
    expect(swatches[0]!.attributes('style')).toContain('var(--chart-1)')
    expect(swatches[1]!.attributes('style')).toContain('var(--chart-2)')
  })

  it('shows "no data" when no recipient is planned', async () => {
    const wrapper = await mountBody({ ...content, recipients: { email: 0, sms: 0, total: 0 } })
    expect(wrapper.find('figure').text()).toContain('Gösterilecek veri yok')
  })
})
