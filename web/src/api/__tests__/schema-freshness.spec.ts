// @vitest-environment node
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

import openapiTS, { astToString, COMMENT_HEADER } from 'openapi-typescript'
import { describe, expect, it } from 'vitest'

// D-084: the committed schema.d.ts must be what `pnpm generate:api` writes from the committed spec.
// Same call as the CLI makes (default options), so the comparison is exact.
const services = ['proposal', 'notification', 'analytics'] as const

describe.each(services)('%s schema.d.ts', (service) => {
  it('is up to date with its swagger.json (run `make generate-web-api`)', async () => {
    const spec = new URL(`../../../../cmd/${service}/apidocs/swagger.json`, import.meta.url)
    const committed = readFileSync(
      fileURLToPath(new URL(`../${service}/schema.d.ts`, import.meta.url)),
      'utf8',
    )

    const generated = COMMENT_HEADER + astToString(await openapiTS(spec))

    expect(committed === generated, `${service}/schema.d.ts is stale`).toBe(true)
  })
})
