import { describe, expect, it } from 'vitest'

import { ApiError, NetworkError, unwrap } from '../errors'

function jsonResponse(status: number, body: unknown, contentType = 'application/problem+json') {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': contentType } })
}

describe('ApiError.from', () => {
  it('reads every field of a problem document, including problems[]', () => {
    const err = ApiError.from(422, {
      type: 'urn:coldbru:proposal:problem:invalid_request',
      title: 'Invalid request',
      status: 422,
      code: 'invalid_request',
      detail: 'The request body is invalid.',
      instance: '/campaigns',
      problems: ['brief: must not be empty', 'limit: must be at most 100'],
    })

    expect(err).toBeInstanceOf(ApiError)
    expect(err).toBeInstanceOf(Error)
    expect(err.status).toBe(422)
    expect(err.code).toBe('invalid_request')
    expect(err.title).toBe('Invalid request')
    expect(err.detail).toBe('The request body is invalid.')
    expect(err.instance).toBe('/campaigns')
    expect(err.problems).toEqual(['brief: must not be empty', 'limit: must be at most 100'])
    expect(err.message).toBe('The request body is invalid.')
  })

  it('prefers the HTTP status over the status field of the body', () => {
    expect(ApiError.from(409, { status: 500, code: 'step_not_open', title: 'x' }).status).toBe(409)
  })

  it('keeps a plain text body as the detail and derives code and title from the status', () => {
    const err = ApiError.from(500, 'internal error\n', 'Internal Server Error')

    expect(err.status).toBe(500)
    expect(err.code).toBe('http_500')
    expect(err.title).toBe('Internal Server Error')
    expect(err.detail).toBe('internal error')
    expect(err.problems).toEqual([])
    expect(err.body).toBe('internal error\n')
  })

  it('truncates a long non-problem body such as a proxy HTML page', () => {
    const err = ApiError.from(502, `<html>${'x'.repeat(5000)}</html>`)

    expect(err.detail).toHaveLength(300)
  })

  it('copes with an empty body, a JSON array and ill-typed fields', () => {
    expect(ApiError.from(404, undefined).title).toBe('HTTP 404')
    expect(ApiError.from(404, undefined).detail).toBeUndefined()
    expect(ApiError.from(400, [1, 2]).code).toBe('http_400')

    const odd = ApiError.from(422, { code: 7, detail: {}, problems: ['a', 3, null] })
    expect(odd.code).toBe('http_422')
    expect(odd.detail).toBeUndefined()
    expect(odd.problems).toEqual(['a'])
  })
})

describe('unwrap', () => {
  it('returns the data of a successful call', async () => {
    const data = { id: 'c1' }

    await expect(unwrap({ data, response: new Response(null, { status: 200 }) })).resolves.toBe(
      data,
    )
  })

  it('accepts a promise of the result', async () => {
    const result = Promise.resolve({ data: 5, response: new Response(null, { status: 200 }) })

    await expect(unwrap(result)).resolves.toBe(5)
  })

  it('returns undefined for a successful call without a body', async () => {
    await expect(unwrap({ response: new Response(null, { status: 204 }) })).resolves.toBeUndefined()
  })

  it('throws an ApiError built from a problem+json answer', async () => {
    const response = jsonResponse(409, {})
    const error = {
      type: 'urn:coldbru:proposal:problem:step_not_open',
      title: 'The step is not open',
      status: 409,
      code: 'step_not_open',
      detail: 'The step is approved; reopen it to change it.',
      instance: '/variants/v1/approve',
    }

    const failure = await unwrap({ error, response }).catch((e: unknown) => e)

    expect(failure).toBeInstanceOf(ApiError)
    expect(failure).toMatchObject({
      status: 409,
      code: 'step_not_open',
      instance: '/variants/v1/approve',
    })
  })

  it('throws an ApiError for a plain text 500', async () => {
    const response = new Response('boom', { status: 500, statusText: 'Internal Server Error' })

    const failure = await unwrap({ error: 'boom', response }).catch((e: unknown) => e)

    expect(failure).toBeInstanceOf(ApiError)
    expect(failure).toMatchObject({ status: 500, code: 'http_500', detail: 'boom' })
  })

  it('throws an ApiError when the failing answer has no body', async () => {
    const failure = await unwrap({ response: new Response(null, { status: 503 }) }).catch(
      (e: unknown) => e,
    )

    expect(failure).toMatchObject({ status: 503, code: 'http_503' })
  })

  it('turns a failed fetch into a NetworkError that keeps the cause', async () => {
    const cause = new TypeError('Failed to fetch')

    const failure = await unwrap(Promise.reject(cause)).catch((e: unknown) => e)

    expect(failure).toBeInstanceOf(NetworkError)
    expect(failure).not.toBeInstanceOf(ApiError)
    expect((failure as NetworkError).cause).toBe(cause)
  })

  it('lets an abort pass through untouched', async () => {
    const abort = new DOMException('aborted', 'AbortError')

    await expect(unwrap(Promise.reject(abort))).rejects.toBe(abort)
  })
})
