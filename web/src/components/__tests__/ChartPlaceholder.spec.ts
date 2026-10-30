import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ChartPlaceholder from '@/components/ChartPlaceholder.vue'
import { createAppI18n } from '@/i18n'

const mountPlaceholder = (props: { error?: unknown } = {}) =>
  mount(ChartPlaceholder, { props, global: { plugins: [createAppI18n()] } })

describe('ChartPlaceholder', () => {
  it('says the chart is loading, as plain status text', () => {
    const wrapper = mountPlaceholder()
    expect(wrapper.text()).toBe('Grafik yükleniyor…')
    expect(wrapper.attributes('role')).toBe('status')
  })

  it('says the chart could not be loaded when the chunk failed, as an alert', () => {
    const wrapper = mountPlaceholder({ error: new Error('Failed to fetch') })
    expect(wrapper.text()).toBe('Grafik yüklenemedi.')
    expect(wrapper.attributes('role')).toBe('alert')
  })
})
