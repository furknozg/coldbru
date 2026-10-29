import { afterEach, describe, expect, it, vi } from 'vitest'

import { serviceBase } from '../client'
import { ApiError, NetworkError, unwrap } from '../errors'
import { notificationEventsUrl, proposalEventsUrl } from '../events'

// Node's Request only takes absolute URLs; a browser resolves the relative base against the page.
// Resolve it the same way here so the call under test is the one the browser would make.
const NodeRequest = globalThis.Request
class PageRequest extends NodeRequest {
  constructor(input: RequestInfo | URL, init?: RequestInit) {
    super(typeof input === 'string' ? new URL(input, window.location.href).href : input, init)
  }
}

function stubFetch(respond: (request: Request) => Response | Promise<Response>) {
  const fetchMock = vi.fn<(input: RequestInfo | URL) => Promise<Response>>(async (input) =>
    respond(input as Request),
  )
  vi.stubGlobal('Request', PageRequest)
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

function sent(fetchMock: { mock: { calls: unknown[][] } }, index = 0): Request {
  return fetchMock.mock.calls[index]![0] as Request
}

// openapi-fetch reads globalThis.Request when a client is created, so load the module (and its
// shared clients) fresh after the stubs are in place.
async function loadClients() {
  vi.resetModules()
  return import('../client')
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('serviceBase', () => {
  it('uses the /api/<service> prefix without a base', () => {
    expect(serviceBase('proposal', undefined)).toBe('/api/proposal')
    expect(serviceBase('notification', '')).toBe('/api/notification')
    expect(serviceBase('analytics', undefined)).toBe('/api/analytics')
  })

  it('puts VITE_API_BASE in front and drops its trailing slash', () => {
    expect(serviceBase('proposal', 'https://gw.example.com/')).toBe(
      'https://gw.example.com/api/proposal',
    )
  })
})

describe('clients', () => {
  it('GET /campaigns of the proposal client hits /api/proposal/campaigns', async () => {
    const fetchMock = stubFetch(
      () =>
        new Response(JSON.stringify({ items: [], total: 0 }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
    )

    const { proposalApi } = await loadClients()

    const data = await unwrap(proposalApi.GET('/campaigns', { params: { query: { limit: 5 } } }))

    expect(data).toEqual({ items: [], total: 0 })
    const request = sent(fetchMock)
    expect(request.method).toBe('GET')
    const url = new URL(request.url)
    expect(url.pathname).toBe('/api/proposal/campaigns')
    expect(url.search).toBe('?limit=5')
  })

  it('substitutes path params and prefixes the other services', async () => {
    const fetchMock = stubFetch(
      () => new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    const { notificationApi, analyticsApi } = await loadClients()
    const id = '019a0e6e-0000-7000-8000-000000000001'

    await notificationApi.GET('/campaigns/{id}/deliveries', { params: { path: { id } } })
    await analyticsApi.GET('/campaigns/{id}/summary', { params: { path: { id } } })

    const paths = fetchMock.mock.calls.map((call) => new URL((call[0] as Request).url).pathname)
    expect(paths).toEqual([
      `/api/notification/campaigns/${id}/deliveries`,
      `/api/analytics/campaigns/${id}/summary`,
    ])
  })

  it('sends a JSON body with POST', async () => {
    const fetchMock = stubFetch(
      () => new Response('{}', { status: 202, headers: { 'Content-Type': 'application/json' } }),
    )

    const { proposalApi } = await loadClients()

    await proposalApi.POST('/campaigns', { body: { brief: 'hello' } })

    const request = sent(fetchMock)
    expect(request.method).toBe('POST')
    expect(new URL(request.url).pathname).toBe('/api/proposal/campaigns')
    expect(await request.json()).toEqual({ brief: 'hello' })
  })

  it('lets an option override the base URL', async () => {
    const fetchMock = stubFetch(() => new Response('{}', { status: 200 }))

    const { createProposalClient } = await loadClients()

    await createProposalClient({ baseUrl: 'http://localhost:8081' }).GET('/healthz')

    expect(sent(fetchMock).url).toBe('http://localhost:8081/healthz')
  })

  it('raises an ApiError with problems[] through unwrap on a 422', async () => {
    stubFetch(
      () =>
        new Response(
          JSON.stringify({
            type: 'urn:coldbru:proposal:problem:invalid_request',
            title: 'Invalid request',
            status: 422,
            code: 'invalid_request',
            problems: ['limit: must be at most 100'],
          }),
          { status: 422, headers: { 'Content-Type': 'application/problem+json' } },
        ),
    )

    const { proposalApi } = await loadClients()

    const failure = await unwrap(
      proposalApi.GET('/campaigns', { params: { query: { limit: 500 } } }),
    ).catch((e: unknown) => e)

    expect(failure).toBeInstanceOf(ApiError)
    expect(failure).toMatchObject({
      status: 422,
      code: 'invalid_request',
      problems: ['limit: must be at most 100'],
    })
  })

  it('raises an ApiError for a plain text 500', async () => {
    stubFetch(() => new Response('upstream exploded', { status: 500 }))

    const { proposalApi } = await loadClients()

    const failure = await unwrap(proposalApi.GET('/campaigns')).catch((e: unknown) => e)

    expect(failure).toMatchObject({ status: 500, code: 'http_500', detail: 'upstream exploded' })
  })

  it('raises a NetworkError when fetch rejects', async () => {
    stubFetch(() => Promise.reject(new TypeError('Failed to fetch')))

    const { proposalApi } = await loadClients()

    await expect(unwrap(proposalApi.GET('/campaigns'))).rejects.toBeInstanceOf(NetworkError)
  })
})

describe('event stream urls', () => {
  it('point at the service prefix', () => {
    expect(proposalEventsUrl('c1')).toBe('/api/proposal/campaigns/c1/events')
    expect(notificationEventsUrl('c1')).toBe('/api/notification/campaigns/c1/deliveries/events')
  })
})
