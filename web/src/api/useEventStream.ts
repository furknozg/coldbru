import {
  getCurrentScope,
  onScopeDispose,
  readonly,
  shallowRef,
  toValue,
  watch,
  type MaybeRefOrGetter,
  type Ref,
} from 'vue'

export type EventStreamStatus = 'connecting' | 'open' | 'closed' | 'error'

/** Handler per event name; the payload type comes from the service's event catalogue. */
export type EventHandlers<Catalog> = Partial<{ [K in keyof Catalog]: (data: Catalog[K]) => void }>

export interface EventStream {
  /**
   * `connecting` while the first connection or a browser reconnect is pending, `open` once
   * connected, `error` after a failed connection (the browser keeps retrying unless the server
   * answered with an error status), `closed` after `close()`, a `null` url or scope disposal.
   */
  readonly status: Readonly<Ref<EventStreamStatus>>
  /** Closes the connection for good; a later url change opens a new one. */
  close: () => void
}

/**
 * Opens an `EventSource` on `url` while it is non-null and delivers each named event's
 * JSON `data` to `handlers[name]` (D-084).
 *
 * - The set of listened events is the handlers' keys when the connection is opened; the handler
 *   itself is looked up on every event.
 * - Reconnecting is the browser's job. The services send a fresh `snapshot` on every
 *   connection (D-072, D-080), so the consumer replaces its state with it.
 * - A stream the server ends on purpose (`done` on the Notification stream) would be reconnected
 *   by the browser: call `close()` from that handler.
 * - The connection closes when the surrounding effect scope is disposed and re-opens when `url`
 *   changes. Data that is not valid JSON is dropped with a console warning.
 */
export function useEventStream<Catalog extends object>(
  url: MaybeRefOrGetter<string | null>,
  handlers: EventHandlers<Catalog>,
): EventStream {
  const status = shallowRef<EventStreamStatus>('closed')
  let source: EventSource | null = null

  function disconnect() {
    source?.close()
    source = null
  }

  function close() {
    disconnect()
    status.value = 'closed'
  }

  function connect(target: string) {
    const es = new EventSource(target)
    source = es
    status.value = 'connecting'
    es.onopen = () => {
      status.value = 'open'
    }
    es.onerror = () => {
      status.value = 'error'
    }
    for (const name of Object.keys(handlers)) {
      es.addEventListener(name, (event) => {
        let data: unknown
        try {
          data = JSON.parse((event as MessageEvent<string>).data)
        } catch {
          console.warn(`useEventStream: "${name}" event has invalid JSON data`)
          return
        }
        const handler = handlers[name as keyof Catalog] as ((payload: unknown) => void) | undefined
        handler?.(data)
      })
    }
  }

  watch(
    () => toValue(url),
    (target) => {
      disconnect()
      if (target === null) status.value = 'closed'
      else connect(target)
    },
    { immediate: true },
  )

  if (getCurrentScope()) onScopeDispose(close)

  return { status: readonly(status), close }
}
