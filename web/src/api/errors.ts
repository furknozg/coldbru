/** RFC 9457 problem document as the services send it (`application/problem+json`, D-072). */
export interface ProblemDocument {
  type?: string
  title?: string
  status?: number
  code?: string
  detail?: string
  instance?: string
  problems?: string[]
}

/** Longest body text kept as `detail` when a non-problem error body is plain text or HTML. */
const MAX_DETAIL_LENGTH = 300

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function asString(value: unknown): string | undefined {
  return typeof value === 'string' && value !== '' ? value : undefined
}

/** The service answered with a non-2xx status. */
export class ApiError extends Error {
  readonly status: number
  /** Stable machine-readable name; `http_<status>` when the body was not a problem document. */
  readonly code: string
  readonly title: string
  readonly detail: string | undefined
  readonly instance: string | undefined
  /** Individual validation messages of an invalid request or invalid content. */
  readonly problems: readonly string[]
  /** The parsed error body exactly as received (object, text or undefined). */
  readonly body: unknown

  constructor(init: {
    status: number
    code: string
    title: string
    detail?: string
    instance?: string
    problems?: readonly string[]
    body?: unknown
  }) {
    super(init.detail ?? init.title)
    this.name = 'ApiError'
    this.status = init.status
    this.code = init.code
    this.title = init.title
    this.detail = init.detail
    this.instance = init.instance
    this.problems = init.problems ?? []
    this.body = init.body
  }

  /**
   * Builds an error from a response status and the body openapi-fetch parsed: a problem
   * document, plain text (proxy or panic pages) or nothing. The HTTP status wins over a
   * `status` field in the body.
   */
  static from(status: number, body: unknown, statusText = ''): ApiError {
    const fallbackTitle = statusText || `HTTP ${status}`
    if (isRecord(body)) {
      const problems = Array.isArray(body.problems)
        ? body.problems.filter((p): p is string => typeof p === 'string')
        : []
      return new ApiError({
        status,
        code: asString(body.code) ?? `http_${status}`,
        title: asString(body.title) ?? fallbackTitle,
        detail: asString(body.detail),
        instance: asString(body.instance),
        problems,
        body,
      })
    }
    const text = typeof body === 'string' ? body.trim() : ''
    return new ApiError({
      status,
      code: `http_${status}`,
      title: fallbackTitle,
      detail: text === '' ? undefined : text.slice(0, MAX_DETAIL_LENGTH),
      body,
    })
  }
}

/** The request never produced a response: offline, DNS, refused connection, CORS. */
export class NetworkError extends Error {
  /** The error fetch rejected with. */
  readonly cause: unknown

  constructor(cause: unknown) {
    super('Network request failed')
    this.name = 'NetworkError'
    this.cause = cause
  }
}

/** What openapi-fetch resolves a call to. */
interface FetchResult<D> {
  data?: D
  error?: unknown
  response: Response
}

/**
 * Returns the data of an openapi-fetch call or throws: `ApiError` for a non-2xx answer,
 * `NetworkError` when fetch itself failed. Aborts and other errors pass through unchanged.
 */
export async function unwrap<D>(result: FetchResult<D> | PromiseLike<FetchResult<D>>): Promise<D> {
  let resolved: FetchResult<D>
  try {
    resolved = await result
  } catch (cause) {
    // fetch rejects with a TypeError when no response arrived; an abort is a DOMException.
    if (cause instanceof TypeError) throw new NetworkError(cause)
    throw cause
  }
  if (!resolved.response.ok) {
    throw ApiError.from(resolved.response.status, resolved.error, resolved.response.statusText)
  }
  return resolved.data as D
}
