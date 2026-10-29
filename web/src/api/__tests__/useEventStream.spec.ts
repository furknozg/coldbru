import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope, nextTick, shallowRef } from 'vue'
import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'

import type { NotificationEvents, ProposalEvents } from '../events'
import { useEventStream } from '../useEventStream'

class FakeEventSource {
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

  listenedEvents() {
    return [...this.listeners.keys()].sort()
  }

  open() {
    this.onopen?.()
  }

  fail() {
    this.onerror?.()
  }

  emit(name: string, data: string) {
    for (const listener of this.listeners.get(name) ?? []) {
      listener(new MessageEvent(name, { data }))
    }
  }
}

const noop = () => {}
const last = () => FakeEventSource.instances[FakeEventSource.instances.length - 1]!

beforeEach(() => {
  FakeEventSource.instances = []
  vi.stubGlobal('EventSource', FakeEventSource)
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('useEventStream', () => {
  it('connects, reports its status and delivers typed events', () => {
    const onSnapshot = vi.fn<(data: NotificationEvents['snapshot']) => void>()
    const onDone = vi.fn<(data: NotificationEvents['done']) => void>()
    const scope = effectScope()
    const stream = scope.run(() =>
      useEventStream<NotificationEvents>('/api/notification/campaigns/c1/deliveries/events', {
        snapshot: onSnapshot,
        done: onDone,
      }),
    )!

    expect(FakeEventSource.instances).toHaveLength(1)
    expect(last().url).toBe('/api/notification/campaigns/c1/deliveries/events')
    expect(last().listenedEvents()).toEqual(['done', 'snapshot'])
    expect(stream.status.value).toBe('connecting')

    last().open()
    expect(stream.status.value).toBe('open')

    last().emit('done', JSON.stringify({ status: 'completed' }))
    expect(onDone).toHaveBeenCalledExactlyOnceWith({ status: 'completed' })
    expect(onSnapshot).not.toHaveBeenCalled()
    scope.stop()
  })

  it('only listens to the events it has handlers for', () => {
    const scope = effectScope()
    scope.run(() =>
      useEventStream<ProposalEvents>('/x', {
        content_delta: vi.fn<(d: ProposalEvents['content_delta']) => void>(),
        ping: vi.fn<(d: ProposalEvents['ping']) => void>(),
      }),
    )

    expect(last().listenedEvents()).toEqual(['content_delta', 'ping'])
    scope.stop()
  })

  it('reports error on a failed connection and open again after the browser reconnects', () => {
    const scope = effectScope()
    const stream = scope.run(() => useEventStream<ProposalEvents>('/x', { ping: noop }))!

    last().open()
    last().fail()
    expect(stream.status.value).toBe('error')
    expect(last().closed).toBe(false)

    last().open()
    expect(stream.status.value).toBe('open')
    expect(FakeEventSource.instances).toHaveLength(1)
    scope.stop()
  })

  it('drops an event whose data is not JSON, and keeps going', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const onPing = vi.fn<(data: ProposalEvents['ping']) => void>()
    const scope = effectScope()
    scope.run(() => useEventStream<ProposalEvents>('/x', { ping: onPing }))

    last().emit('ping', 'not json')
    expect(onPing).not.toHaveBeenCalled()
    expect(warn).toHaveBeenCalledOnce()

    last().emit('ping', JSON.stringify({ at: '2026-10-30T10:00:00Z' }))
    expect(onPing).toHaveBeenCalledExactlyOnceWith({ at: '2026-10-30T10:00:00Z' })
    scope.stop()
  })

  it('close() closes the connection for good, also when called from a handler', () => {
    const scope = effectScope()
    const stream = scope.run(() => {
      const s = useEventStream<NotificationEvents>('/x', { done: () => s.close() })
      return s
    })!

    last().open()
    last().emit('done', JSON.stringify({ status: 'completed' }))

    expect(last().closed).toBe(true)
    expect(stream.status.value).toBe('closed')
    scope.stop()
  })

  it('does not connect while the url is null, and connects when it appears', async () => {
    const url = shallowRef<string | null>(null)
    const scope = effectScope()
    const stream = scope.run(() => useEventStream<ProposalEvents>(url, { ping: noop }))!

    expect(FakeEventSource.instances).toHaveLength(0)
    expect(stream.status.value).toBe('closed')

    url.value = '/api/proposal/campaigns/c1/events'
    await nextTick()
    expect(FakeEventSource.instances).toHaveLength(1)
    expect(stream.status.value).toBe('connecting')
    scope.stop()
  })

  it('reopens on a url change and closes the previous connection', async () => {
    const url = shallowRef<string | null>('/a')
    const onPing = vi.fn<(data: ProposalEvents['ping']) => void>()
    const scope = effectScope()
    const stream = scope.run(() => useEventStream<ProposalEvents>(url, { ping: onPing }))!
    const first = last()
    first.open()

    url.value = '/b'
    await nextTick()

    expect(first.closed).toBe(true)
    expect(FakeEventSource.instances).toHaveLength(2)
    expect(last().url).toBe('/b')
    expect(stream.status.value).toBe('connecting')

    url.value = null
    await nextTick()
    expect(last().closed).toBe(true)
    expect(stream.status.value).toBe('closed')
    scope.stop()
  })

  it('accepts a getter as the url', async () => {
    const id = shallowRef('c1')
    const scope = effectScope()
    scope.run(() => useEventStream<ProposalEvents>(() => `/campaigns/${id.value}/events`, {}))
    expect(last().url).toBe('/campaigns/c1/events')

    id.value = 'c2'
    await nextTick()
    expect(last().url).toBe('/campaigns/c2/events')
    scope.stop()
  })

  it('closes when the component unmounts', () => {
    const Probe = defineComponent({
      setup() {
        useEventStream<ProposalEvents>('/x', { ping: noop })
        return () => h('div')
      },
    })

    const wrapper = mount(Probe)
    expect(last().closed).toBe(false)

    wrapper.unmount()
    expect(last().closed).toBe(true)
  })
})
