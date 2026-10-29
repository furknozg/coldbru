import createClient, { type ClientOptions } from 'openapi-fetch'

import type { paths as AnalyticsPaths } from './analytics/schema'
import type { paths as NotificationPaths } from './notification/schema'
import type { paths as ProposalPaths } from './proposal/schema'

/**
 * Path prefix of each service in the browser (D-084). nginx (T-034) and the Vite dev proxy strip it
 * before forwarding, so the paths in the generated schemas stay the services' own.
 */
export const SERVICE_PREFIX = {
  proposal: '/api/proposal',
  notification: '/api/notification',
  analytics: '/api/analytics',
} as const

export type Service = keyof typeof SERVICE_PREFIX

/** Base URL of a service: `VITE_API_BASE` (when set) + `/api/<service>`, no trailing slash. */
export function serviceBase(service: Service, apiBase = import.meta.env.VITE_API_BASE): string {
  return `${(apiBase ?? '').replace(/\/+$/, '')}${SERVICE_PREFIX[service]}`
}

/** Looks `fetch` up at call time so a replaced `globalThis.fetch` (tests, polyfills) is honoured. */
const lateFetch: NonNullable<ClientOptions['fetch']> = (request) => globalThis.fetch(request)

function create<P extends object>(service: Service, options: ClientOptions) {
  return createClient<P>({ baseUrl: serviceBase(service), fetch: lateFetch, ...options })
}

export const createProposalClient = (options: ClientOptions = {}) =>
  create<ProposalPaths>('proposal', options)
export const createNotificationClient = (options: ClientOptions = {}) =>
  create<NotificationPaths>('notification', options)
export const createAnalyticsClient = (options: ClientOptions = {}) =>
  create<AnalyticsPaths>('analytics', options)

/** One shared instance per service; wrap calls in `unwrap()` for `ApiError` / `NetworkError`. */
export const proposalApi = createProposalClient()
export const notificationApi = createNotificationClient()
export const analyticsApi = createAnalyticsClient()
