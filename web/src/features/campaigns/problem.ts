import { ApiError, NetworkError } from '@/api/errors'

/** What the UI shows of a failed call: one message, the validation lines and the HTTP status. */
export interface Problem {
  /** HTTP status; `0` when no response arrived. */
  status: number
  code: string
  message: string
  problems: readonly string[]
  network: boolean
}

type Translate = (key: string) => string

/**
 * Turns whatever a service call threw into a `Problem` (D-084): the `detail` of the problem+json
 * wins over its title, a missing response becomes the retry hint.
 */
export function toProblem(error: unknown, t: Translate): Problem {
  if (error instanceof ApiError) {
    return {
      status: error.status,
      code: error.code,
      message: error.detail ?? error.title,
      problems: error.problems,
      network: false,
    }
  }
  if (error instanceof NetworkError) {
    return { status: 0, code: 'network', message: t('errors.network'), problems: [], network: true }
  }
  return { status: 0, code: 'unknown', message: t('errors.unknown'), problems: [], network: false }
}
