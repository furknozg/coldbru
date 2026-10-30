import { describe, expect, it } from 'vitest'

import { createAppI18n, LOCALE } from '@/i18n'
import tr from '@/locales/tr.json'
import { CHANNELS, PLAN_MODES } from '@/features/campaigns/content'
import { CAMPAIGN_STATUSES, STEP_ORDER } from '@/features/campaigns/types'
import { PRESETS } from '@/features/dashboard/dateRange'
import { LIVE_STATUSES } from '@/features/dashboard/live'
import { APP_MENU, MENU_LINKS } from '@/features/dashboard/menu'
import { PURPOSES } from '@/features/dashboard/series'

/** Every source file of the app, as text; tests and generated schemas are not part of it. */
const sources = Object.entries(
  import.meta.glob<string>(
    ['/src/**/*.vue', '/src/**/*.ts', '!/src/**/__tests__/**', '!/src/**/schema.d.ts'],
    { query: '?raw', import: 'default', eager: true },
  ),
)

function has(key: string): boolean {
  let node: unknown = tr
  for (const part of key.split('.')) {
    if (typeof node !== 'object' || node === null || !(part in node)) return false
    node = (node as Record<string, unknown>)[part]
  }
  return typeof node === 'string'
}

/** Keys that are built at run time, with every value the builder can put in. */
const DYNAMIC: Record<string, readonly string[]> = {
  'step.kind.': STEP_ORDER,
  'step.status.': ['open', 'approved', 'superseded', 'waiting'],
  'campaign.status.': CAMPAIGN_STATUSES,
  'list.group.': CAMPAIGN_STATUSES,
  'variant.status.': ['draft', 'approved', 'rejected', 'superseded'],
  'channel.': CHANNELS,
  'plan.mode.': PLAN_MODES,
  'audience.coverageKind.': ['both', 'email_only', 'sms_only', 'unreachable'],
  'timeline.campaignStatus.': ['sending', 'completed'],
  'timeline.screen.': ['allow', 'block', 'error'],
  'dashboard.range.': PRESETS,
  'dashboard.purpose.': PURPOSES,
  'dashboard.deliveryStatus.': LIVE_STATUSES,
  'dashboard.live.done.': ['completed', 'cancelled'],
  // Looked up with `te` first, so a language without a name falls back to its code.
  'language.': ['tr', 'en'],
}

const CALL = /(?:\bt|\bte|\$t)\(\s*(['"`])((?:(?!\1)[^\\])*)\1/g

function usedKeys() {
  const fixed = new Map<string, string>()
  const dynamic = new Map<string, string>()
  for (const [path, text] of sources) {
    for (const match of text.matchAll(CALL)) {
      const key = match[2]!
      if (key.includes('${')) dynamic.set(key.slice(0, key.indexOf('${')), path)
      else fixed.set(key, path)
    }
  }
  return { fixed, dynamic }
}

describe('Turkish strings', () => {
  it('finds the source files and the calls it checks', () => {
    const { fixed, dynamic } = usedKeys()
    expect(sources.length).toBeGreaterThan(30)
    expect(fixed.size).toBeGreaterThan(100)
    expect(dynamic.size).toBeGreaterThan(5)
  })

  it('has every key the code asks for', () => {
    const missing = [...usedKeys().fixed]
      .filter(([key]) => !has(key))
      .map(([key, path]) => `${key} (${path})`)
    expect(missing).toEqual([])
  })

  it('has every value of the keys built at run time', () => {
    const { dynamic } = usedKeys()
    const unknownPrefix = [...dynamic]
      .filter(([prefix]) => !(prefix in DYNAMIC))
      .map(([p, path]) => `${p} (${path})`)
    expect(unknownPrefix, 'list the values of a new dynamic key in DYNAMIC').toEqual([])

    const missing = Object.entries(DYNAMIC).flatMap(([prefix, values]) =>
      values.map((v) => `${prefix}${v}`).filter((key) => !has(key)),
    )
    expect(missing).toEqual([])
  })

  it('has a label for every link and group of the menu', () => {
    const groups = APP_MENU.flatMap((entry) => (entry.kind === 'group' ? [entry.label] : []))
    const missing = [...MENU_LINKS.map((item) => item.label), ...groups].filter((key) => !has(key))
    expect(missing).toEqual([])
  })

  it('renders a key and reports a missing one as missing, not as empty', () => {
    const { global } = createAppI18n()
    expect(global.locale.value).toBe(LOCALE)
    expect(global.t('variant.approve')).toBe('Onayla')
    expect(global.te('no.such.key')).toBe(false)
  })

  it('keeps Turkish text out of components and code', () => {
    // Comments are English; what is left must hold no Turkish letter: user text lives in tr.json.
    const withTurkish = sources
      .filter(([, text]) => {
        const code = text
          .replace(/<!--[\s\S]*?-->/g, '')
          .replace(/\/\*[\s\S]*?\*\//g, '')
          .replace(/(^|[^:'"`])\/\/.*$/gm, '$1')
        return /[çğıöşüÇĞİÖŞÜ]/.test(code)
      })
      .map(([path]) => path)
    expect(withTurkish).toEqual([])
  })

  it('formats dates and money the Turkish way', () => {
    const { global } = createAppI18n()
    expect(global.n(1234, 'int')).toBe('1.234')
    expect(global.n(0.012345, 'usd')).toBe('$0,012345')
    expect(global.n(1.04, 'usd')).toBe('$1,04')
    expect(global.d('2026-10-30T10:00:00Z', { key: 'time', timeZone: 'UTC' })).toBe('10:00')
    expect(global.d('2026-11-01T06:30:00Z', { key: 'zoned', timeZone: 'Europe/Istanbul' })).toMatch(
      /1 Kas 2026.*09:30/,
    )
  })
})
