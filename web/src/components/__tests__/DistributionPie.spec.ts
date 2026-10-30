import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import DistributionPie from '@/components/DistributionPie.vue'
import { createAppI18n } from '@/i18n'
import type { Slice } from '@/features/campaigns/distribution'

const slices: Slice[] = [
  { key: 'a', label: 'Alfa', value: 750 },
  { key: 'b', label: 'Beta', value: 250 },
  { key: 'c', label: 'Gama', value: 0 },
]

const mountPie = (props: { title: string; slices: Slice[] }) =>
  mount(DistributionPie, { props, global: { plugins: [createAppI18n()] } })

describe('DistributionPie', () => {
  it('lists every slice with its count and share as text', () => {
    const wrapper = mountPie({ title: 'Dağılım', slices })
    const rows = wrapper.findAll('li').map((li) => li.findAll('span').map((span) => span.text()))
    // label, count, share
    expect(rows).toEqual([
      ['Alfa', '750', '%75'],
      ['Beta', '250', '%25'],
      ['Gama', '0', '%0'],
    ])
    expect(wrapper.find('figcaption').text()).toBe('Dağılım')
  })

  it('formats counts and shares the Turkish way', () => {
    const wrapper = mountPie({
      title: 'T',
      slices: [
        { key: 'a', label: 'A', value: 1234 },
        { key: 'b', label: 'B', value: 1 },
      ],
    })
    expect(wrapper.findAll('li')[0]!.text()).toContain('1.234')
    expect(wrapper.findAll('li')[0]!.text()).toContain('%99,9')
  })

  it('draws an arc only for slices with a value and keeps the others in the legend', () => {
    const wrapper = mountPie({ title: 'Dağılım', slices })
    expect(wrapper.get('[data-testid="container"]').attributes('data-arcs')).toBe('2')
    expect(wrapper.findAll('li')).toHaveLength(3)
  })

  it('names the chart and its numbers for assistive technology', () => {
    const wrapper = mountPie({ title: 'Dağılım', slices })
    const chart = wrapper.get('[role="img"]')
    expect(chart.attributes('aria-label')).toBe(
      'Dağılım: Alfa: 750 (%75), Beta: 250 (%25), Gama: 0 (%0)',
    )
  })

  it('gives each slice its own color and passes it to the swatch', () => {
    const wrapper = mountPie({
      title: 'T',
      slices: [
        { key: 'a', label: 'A', value: 1, color: 'rgb(1, 2, 3)' },
        { key: 'b', label: 'B', value: 1 },
      ],
    })
    const swatches = wrapper.findAll('li > svg circle')
    expect(swatches[0]!.attributes('style')).toContain('rgb(1, 2, 3)')
    expect(swatches[1]!.attributes('style')).toContain('var(--chart-2)')
  })

  it('shows plain text instead of a chart when every slice is zero', () => {
    const wrapper = mountPie({
      title: 'Dağılım',
      slices: [
        { key: 'a', label: 'A', value: 0 },
        { key: 'b', label: 'B', value: 0 },
      ],
    })
    expect(wrapper.text()).toContain('Gösterilecek veri yok')
    expect(wrapper.find('[role="img"]').exists()).toBe(false)
    expect(wrapper.find('li').exists()).toBe(false)
  })

  it('has no card or border of its own (D-091)', () => {
    const wrapper = mountPie({ title: 'Dağılım', slices })
    const boxy = /^(border(-\S+)?|rounded-(md|lg|xl)|shadow(-\S+)?|bg-(card|muted|accent))$/
    const classes = [wrapper.element, ...wrapper.element.querySelectorAll('*')].flatMap((el) => [
      ...el.classList,
    ])
    expect(classes.filter((name) => boxy.test(name))).toEqual([])
  })
})
