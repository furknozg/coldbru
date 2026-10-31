import {
  CalendarDateTime,
  parseAbsolute,
  toCalendarDate,
  toZoned,
  type CalendarDate,
  type DateValue,
  type ZonedDateTime,
} from '@internationalized/date'

/**
 * The wall clock of a schedule slot (D-119): a send time is edited in the IANA zone of its slot,
 * and goes to the server as RFC 3339 with the offset that zone has at that instant, daylight saving
 * included. The server checks exactly that (`flow.ScheduleProblems`).
 */

const pad = (n: number, width = 2) => String(n).padStart(width, '0')

/** The instant of `sendAt` (RFC 3339) as a wall clock in `timeZone`; null when either is unreadable. */
export function zonedOf(sendAt: string, timeZone: string): ZonedDateTime | null {
  try {
    return parseAbsolute(sendAt, timeZone)
  } catch {
    return null
  }
}

/** "HH:mm" of a zoned instant, as the time field shows it. */
export function timeOf(zoned: ZonedDateTime): string {
  return `${pad(zoned.hour)}:${pad(zoned.minute)}`
}

/** The day of a date value of any kind, as the calendar holds it. */
export function dateOf(value: DateValue): CalendarDate {
  return toCalendarDate(value)
}

/** A day and an "HH:mm" wall-clock time in `timeZone`; null when the time does not parse. */
export function zonedFrom(
  date: CalendarDate,
  time: string,
  timeZone: string,
): ZonedDateTime | null {
  const match = /^(\d{2}):(\d{2})(?::\d{2})?$/.exec(time)
  if (!match) return null
  const hour = Number(match[1])
  const minute = Number(match[2])
  if (hour > 23 || minute > 59) return null
  try {
    return toZoned(new CalendarDateTime(date.year, date.month, date.day, hour, minute), timeZone)
  } catch {
    return null
  }
}

/** "2026-07-01T10:00:00+01:00": the wall clock of the zone with its offset, no zone name. */
export function rfc3339(zoned: ZonedDateTime): string {
  const total = Math.round(zoned.offset / 60_000)
  const sign = total < 0 ? '-' : '+'
  const abs = Math.abs(total)
  const offset = `${sign}${pad(Math.floor(abs / 60))}:${pad(abs % 60)}`
  return (
    `${pad(zoned.year, 4)}-${pad(zoned.month)}-${pad(zoned.day)}` +
    `T${pad(zoned.hour)}:${pad(zoned.minute)}:${pad(zoned.second)}${offset}`
  )
}
