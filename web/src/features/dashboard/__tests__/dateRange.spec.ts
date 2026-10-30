import { describe, expect, it } from 'vitest'

import {
  ALL_TIME,
  activePreset,
  formatDate,
  parseDate,
  presetRange,
  rangeFromQuery,
  rangeToQuery,
  seriesQuery,
  toApiRange,
} from '../dateRange'

// Dates are the viewer's local calendar days, so the expected instants are built the same way and
// the tests hold in any time zone.
const NOW = new Date(2026, 9, 30, 15, 20)
const utc = (year: number, month: number, day: number) =>
  new Date(year, month - 1, day).toISOString().replace(/\.000Z$/, 'Z')

describe('dates', () => {
  it('writes and reads YYYY-MM-DD, and rejects what is no calendar day', () => {
    expect(formatDate(new Date(2026, 0, 5))).toBe('2026-01-05')
    expect(parseDate('2026-10-30')).toEqual(new Date(2026, 9, 30))
    expect(parseDate('2026-02-30')).toBeNull()
    expect(parseDate('30.10.2026')).toBeNull()
    expect(parseDate('')).toBeNull()
  })
})

describe('presets', () => {
  it('count the days back from today, today included', () => {
    expect(presetRange('today', NOW)).toEqual({ from: '2026-10-30', to: '2026-10-30' })
    expect(presetRange('last7', NOW)).toEqual({ from: '2026-10-24', to: '2026-10-30' })
    expect(presetRange('last30', NOW)).toEqual({ from: '2026-10-01', to: '2026-10-30' })
    expect(presetRange('last90', NOW)).toEqual({ from: '2026-08-02', to: '2026-10-30' })
  })

  it('make "Tümü" the empty range', () => {
    expect(presetRange('all', NOW)).toEqual(ALL_TIME)
    expect(rangeToQuery(ALL_TIME)).toEqual({})
  })

  it('are recognised from a range, and a custom range matches none', () => {
    for (const preset of ['today', 'last7', 'last30', 'last90', 'all'] as const) {
      expect(activePreset(presetRange(preset, NOW), NOW)).toBe(preset)
    }
    expect(activePreset({ from: '2026-10-10', to: '2026-10-12' }, NOW)).toBeNull()
    // A "last 7 days" link opened a day later is a custom range now.
    expect(activePreset(presetRange('last7', NOW), new Date(2026, 9, 31))).toBeNull()
  })
})

describe('the range in the URL', () => {
  it('reads from and to', () => {
    expect(rangeFromQuery({ from: '2026-10-01', to: '2026-10-30' })).toEqual({
      from: '2026-10-01',
      to: '2026-10-30',
    })
    expect(rangeFromQuery({})).toEqual(ALL_TIME)
    expect(rangeToQuery({ from: '2026-10-01', to: '2026-10-30' })).toEqual({
      from: '2026-10-01',
      to: '2026-10-30',
    })
  })

  it('drops a bound that is not a date and swaps a backwards range', () => {
    expect(rangeFromQuery({ from: 'dün', to: '2026-10-30' })).toEqual({
      from: null,
      to: '2026-10-30',
    })
    expect(rangeFromQuery({ from: ['2026-10-01', 'x'], to: undefined })).toEqual({
      from: '2026-10-01',
      to: null,
    })
    expect(rangeFromQuery({ from: '2026-10-30', to: '2026-10-01' })).toEqual({
      from: '2026-10-01',
      to: '2026-10-30',
    })
  })
})

describe('API range', () => {
  it('is [start of from, start of the day after to) in RFC 3339 UTC with a Z', () => {
    const api = toApiRange({ from: '2026-10-24', to: '2026-10-30' })
    expect(api).toEqual({ from: utc(2026, 10, 24), to: utc(2026, 10, 31) })
    expect(api.from).toMatch(/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ$/)
    expect(api.from).not.toContain('+')
  })

  it('leaves an open bound out of the request', () => {
    expect(toApiRange(ALL_TIME)).toEqual({})
    expect(toApiRange({ from: '2026-10-24', to: null })).toEqual({ from: utc(2026, 10, 24) })
    expect(toApiRange({ from: null, to: '2026-10-24' })).toEqual({ to: utc(2026, 10, 25) })
  })

  it('makes one day wide for a single-day range', () => {
    expect(toApiRange({ from: '2026-10-30', to: '2026-10-30' })).toEqual({
      from: utc(2026, 10, 30),
      to: utc(2026, 10, 31),
    })
  })
})

describe('series window and bucket', () => {
  it('draws today and yesterday-to-today by hour', () => {
    expect(seriesQuery(presetRange('today', NOW), NOW)).toEqual({
      from: utc(2026, 10, 30),
      to: utc(2026, 10, 31),
      bucket: 'hour',
    })
    expect(seriesQuery({ from: '2026-10-29', to: '2026-10-30' }, NOW).bucket).toBe('hour')
  })

  it('draws longer ranges by day', () => {
    const week = seriesQuery(presetRange('last7', NOW), NOW)
    expect(week).toEqual({ from: utc(2026, 10, 24), to: utc(2026, 10, 31), bucket: 'day' })
    expect(seriesQuery({ from: '2026-10-28', to: '2026-10-30' }, NOW).bucket).toBe('day')
    expect(seriesQuery(presetRange('last90', NOW), NOW).bucket).toBe('day')
  })

  it('draws "Tümü" as the last 90 days ending tomorrow, by day', () => {
    expect(seriesQuery(ALL_TIME, NOW)).toEqual({
      from: utc(2026, 8, 2),
      to: utc(2026, 10, 31),
      bucket: 'day',
    })
  })

  it('fills an open end: no start is 90 days before the end, no end is tomorrow', () => {
    expect(seriesQuery({ from: null, to: '2026-10-10' }, NOW)).toEqual({
      from: utc(2026, 7, 13),
      to: utc(2026, 10, 11),
      bucket: 'day',
    })
    expect(seriesQuery({ from: '2026-10-20', to: null }, NOW)).toEqual({
      from: utc(2026, 10, 20),
      to: utc(2026, 10, 31),
      bucket: 'day',
    })
  })

  it('keeps the latest 400 days when the range is longer than the API allows', () => {
    const query = seriesQuery({ from: '2020-01-01', to: '2026-10-30' }, NOW)
    expect(query.to).toBe(utc(2026, 10, 31))
    expect(query.from).toBe(utc(2025, 9, 26))
    expect(query.bucket).toBe('day')
  })
})
