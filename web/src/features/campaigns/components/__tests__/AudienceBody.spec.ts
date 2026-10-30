import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import { audienceContent, variantOf } from '@/__tests__/helpers'
import { createAppI18n } from '@/i18n'

import AudienceBody from '../AudienceBody.vue'

/** The pies are a lazy chunk (D-092): the body is mounted once it has loaded. */
async function mountBody(content: Record<string, unknown>) {
  const wrapper = mount(AudienceBody, {
    props: { variant: variantOf({ kind: 'audience', content }) },
    global: { plugins: [createAppI18n()] },
  })
  await vi.dynamicImportSettled()
  await flushPromises()
  return wrapper
}

/** The legend of the pie under `title`, as [label, count, share] rows. */
function legend(wrapper: Awaited<ReturnType<typeof mountBody>>, title: string) {
  const figure = wrapper.findAll('figure').find((f) => f.find('figcaption').text() === title)
  if (!figure) throw new Error(`no pie titled ${title}`)
  return figure.findAll('li').map((li) => li.findAll('span').map((s) => s.text()))
}

describe('AudienceBody', () => {
  it('shows the total and the three pies instead of tables, and not the rule (D-096)', async () => {
    const wrapper = await mountBody(audienceContent)
    expect(wrapper.text()).toContain('48')
    expect(wrapper.find('pre').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('orders.none')
    expect(wrapper.text()).not.toContain('city ===')
    expect(wrapper.findAll('figure').map((f) => f.find('figcaption').text())).toEqual([
      'Kanal kapsaması',
      'Dil',
      'Yerel ayar',
    ])
    expect(wrapper.find('table').exists()).toBe(false)
  })

  it('charts the channel coverage with the four kinds in a fixed order', async () => {
    const wrapper = await mountBody(audienceContent)
    expect(legend(wrapper, 'Kanal kapsaması')).toEqual([
      ['İkisi de', '22', '%43,1'],
      ['Yalnız e-posta', '13', '%25,5'],
      ['Yalnız SMS', '13', '%25,5'],
      ['Ulaşılamaz', '3', '%5,9'],
    ])
  })

  it('sums by_locale per language for the language pie and lists the locales', async () => {
    const wrapper = await mountBody({
      ...audienceContent,
      by_locale: [
        {
          language: 'tr',
          timezone: 'Europe/Istanbul',
          count: 30,
          coverage: audienceContent.coverage,
        },
        {
          language: 'tr',
          timezone: 'Europe/Berlin',
          count: 12,
          coverage: audienceContent.coverage,
        },
        { language: 'en', timezone: 'Europe/London', count: 6, coverage: audienceContent.coverage },
      ],
    })
    expect(legend(wrapper, 'Dil')).toEqual([
      ['Türkçe', '42', '%87,5'],
      ['İngilizce', '6', '%12,5'],
    ])
    expect(legend(wrapper, 'Yerel ayar')).toEqual([
      ['Türkçe · Europe/Istanbul', '30', '%62,5'],
      ['Türkçe · Europe/Berlin', '12', '%25'],
      ['İngilizce · Europe/London', '6', '%12,5'],
    ])
  })

  it('uses by_language and hides the locale pie for a variant from before D-076', async () => {
    const wrapper = await mountBody({ ...audienceContent, by_locale: undefined })
    expect(legend(wrapper, 'Dil')).toEqual([
      ['Türkçe', '42', '%87,5'],
      ['İngilizce', '6', '%12,5'],
    ])
    expect(wrapper.findAll('figure').map((f) => f.find('figcaption').text())).toEqual([
      'Kanal kapsaması',
      'Dil',
    ])
  })

  it('draws no arc for a kind nobody is in, but keeps it in the legend', async () => {
    const wrapper = await mountBody({
      ...audienceContent,
      coverage: { both: 10, email_only: 0, sms_only: 0, unreachable: 0 },
    })
    const figure = wrapper.findAll('figure')[0]!
    expect(figure.get('[data-testid="container"]').attributes('data-arcs')).toBe('1')
    expect(figure.findAll('li')).toHaveLength(4)
  })

  it('shows "no data" for a variant with an empty audience', async () => {
    const wrapper = await mountBody({})
    expect(wrapper.text()).toContain('Gösterilecek veri yok')
    expect(wrapper.find('table').exists()).toBe(false)
  })
})
