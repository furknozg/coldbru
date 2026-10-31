import { CalendarDate } from '@internationalized/date'
import { describe, expect, it } from 'vitest'

import { dateOf, rfc3339, timeOf, zonedFrom, zonedOf } from '../schedule'

describe('schedule wall clock (D-119)', () => {
  it('writes the offset of the slot zone at that instant, daylight saving included', () => {
    const london = (month: number) =>
      rfc3339(zonedFrom(new CalendarDate(2026, month, 1), '10:00', 'Europe/London')!)
    expect(london(7)).toBe('2026-07-01T10:00:00+01:00')
    expect(london(1)).toBe('2026-01-01T10:00:00+00:00')
    expect(rfc3339(zonedFrom(new CalendarDate(2026, 11, 1), '09:30', 'Europe/Istanbul')!)).toBe(
      '2026-11-01T09:30:00+03:00',
    )
    expect(rfc3339(zonedFrom(new CalendarDate(2026, 2, 3), '08:05', 'America/New_York')!)).toBe(
      '2026-02-03T08:05:00-05:00',
    )
  })

  it('reads an instant back as the wall clock of the zone', () => {
    const zoned = zonedOf('2026-11-01T06:30:00Z', 'Europe/Istanbul')!
    expect(dateOf(zoned).toString()).toBe('2026-11-01')
    expect(timeOf(zoned)).toBe('09:30')
    expect(zonedOf('2026-11-01T06:30:00Z', 'Mars/Olympus')).toBeNull()
    expect(zonedOf('yesterday', 'Europe/Istanbul')).toBeNull()
  })

  it('refuses a time that is not HH:mm', () => {
    const day = new CalendarDate(2026, 11, 1)
    expect(zonedFrom(day, '', 'Europe/Istanbul')).toBeNull()
    expect(zonedFrom(day, '25:00', 'Europe/Istanbul')).toBeNull()
    expect(zonedFrom(day, '9:30', 'Europe/Istanbul')).toBeNull()
    expect(rfc3339(zonedFrom(day, '09:30:00', 'Europe/Istanbul')!)).toBe(
      '2026-11-01T09:30:00+03:00',
    )
  })
})
