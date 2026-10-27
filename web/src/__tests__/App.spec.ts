import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

import App from '../App.vue'

describe('App', () => {
  it('renders the app shell with a router view', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [] })
    const wrapper = mount(App, { global: { plugins: [router] } })
    await router.isReady()

    expect(wrapper.get('h1').text()).toBe('Smart Campaign Composer')
  })
})
