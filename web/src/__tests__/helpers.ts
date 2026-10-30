import { flushPromises, mount } from '@vue/test-utils'
import { vi } from 'vitest'
import { createMemoryHistory } from 'vue-router'

import App from '@/App.vue'
import { createAppI18n } from '@/i18n'
import { createAppRouter } from '@/router'
import type {
  CampaignInfo,
  CampaignView,
  StepKind,
  StepView,
  VariantView,
} from '@/features/campaigns/types'

/** Stands in for the browser's `EventSource`: tests open, emit and inspect the connections. */
export class FakeEventSource {
  static instances: FakeEventSource[] = []

  onopen: (() => void) | null = null
  onerror: (() => void) | null = null
  closed = false
  private listeners = new Map<string, Array<(event: MessageEvent<string>) => void>>()

  constructor(readonly url: string) {
    FakeEventSource.instances.push(this)
  }

  addEventListener(name: string, listener: (event: MessageEvent<string>) => void) {
    this.listeners.set(name, [...(this.listeners.get(name) ?? []), listener])
  }

  close() {
    this.closed = true
  }

  /** Delivers an event the way the server would: a name and a JSON `data` string. */
  emit(name: string, data: unknown) {
    for (const listener of this.listeners.get(name) ?? []) {
      listener(new MessageEvent(name, { data: JSON.stringify(data) }))
    }
  }

  static get open() {
    return FakeEventSource.instances.filter((es) => !es.closed)
  }

  static get last() {
    return FakeEventSource.instances[FakeEventSource.instances.length - 1]!
  }

  static install() {
    FakeEventSource.instances = []
    vi.stubGlobal('EventSource', FakeEventSource)
  }
}

let counter = 0
const uuid = () => `00000000-0000-4000-8000-${String(++counter).padStart(12, '0')}`
const NOW = '2026-10-30T10:00:00.000Z'

/** `content` is typed `Record<string, never>` in the generated schema; tests write real shapes. */
type VariantOverrides = Omit<Partial<VariantView>, 'content'> & {
  content?: Record<string, unknown>
}

export function variantOf(overrides: VariantOverrides = {}): VariantView {
  return {
    id: uuid(),
    step_id: uuid(),
    kind: 'audience',
    status: 'draft',
    created_at: NOW,
    content: {},
    ...overrides,
  } as VariantView
}

export const audienceContent = {
  dsl: 'city === "İstanbul" && orders.none(o => o.orderDate >= ago("30d"))',
  count: 48,
  coverage: { both: 22, email_only: 13, sms_only: 13, unreachable: 3 },
  by_language: [
    {
      language: 'tr',
      count: 42,
      coverage: { both: 18, email_only: 11, sms_only: 13, unreachable: 2 },
    },
    { language: 'en', count: 6, coverage: { both: 4, email_only: 2, sms_only: 0, unreachable: 1 } },
  ],
  by_locale: [
    {
      language: 'tr',
      timezone: 'Europe/Istanbul',
      count: 42,
      coverage: { both: 18, email_only: 11, sms_only: 13, unreachable: 2 },
    },
    {
      language: 'en',
      timezone: 'Europe/London',
      count: 6,
      coverage: { both: 4, email_only: 2, sms_only: 0, unreachable: 1 },
    },
  ],
}

export function stepOf(kind: StepKind, overrides: Partial<StepView> = {}): StepView {
  const id = overrides.id ?? uuid()
  return {
    id,
    campaign_id: 'c1',
    kind,
    version: 1,
    status: 'open',
    created_at: NOW,
    failed: false,
    generating: false,
    variants: [],
    ...overrides,
  } as StepView
}

export function viewOf(overrides: Partial<CampaignView> = {}): CampaignView {
  return {
    id: 'c1',
    brief: 'İstanbul için %30 indirim hatırlatması',
    status: 'drafting',
    created_at: NOW,
    updated_at: NOW,
    steps: [],
    notes: [],
    messages: [],
    cost: { sms_usd: '0.0000', email_usd: '0.0000', total_usd: '0.0000' },
    ...overrides,
  }
}

export function infoOf(overrides: Partial<CampaignInfo> = {}): CampaignInfo {
  return {
    id: uuid(),
    brief: 'Bir brief',
    status: 'drafting',
    created_at: NOW,
    updated_at: NOW,
    ...overrides,
  }
}

/** The whole app on an in-memory history, with the real routes, i18n and shell. */
export async function mountApp(path: string) {
  const router = createAppRouter(createMemoryHistory())
  await router.push(path)
  await router.isReady()
  const wrapper = mount(App, {
    global: { plugins: [router, createAppI18n()] },
    attachTo: document.body,
  })
  await flushPromises()
  return { wrapper, router }
}

/** Dialogs render into `document.body`, outside the wrapper. */
export const dialog = () => document.body.querySelector<HTMLElement>('[role="dialog"]')
