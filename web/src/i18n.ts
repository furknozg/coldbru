import { createI18n } from 'vue-i18n'

import tr from './locales/tr.json'

export const LOCALE = 'tr'

/**
 * Dates and numbers follow tr-TR; amounts are US dollars (D-083). The money formats keep up to
 * six decimals because a model call costs fractions of a cent. `zoned` names the time zone, for
 * send times shown in the zone of their slot.
 */
const datetimeFormats = {
  [LOCALE]: {
    short: { dateStyle: 'medium', timeStyle: 'short' },
    // The date alone, for a row of the campaign list (D-091).
    date: { dateStyle: 'medium' },
    time: { hour: '2-digit', minute: '2-digit' },
    // Intl rejects timeZoneName together with dateStyle/timeStyle, so the fields are spelled out.
    zoned: {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      timeZoneName: 'short',
    },
  },
} as const

const numberFormats = {
  [LOCALE]: {
    int: { maximumFractionDigits: 0 },
    // A share of a whole, as in the legend of a pie chart (D-092): %45,8.
    percent: { style: 'percent', maximumFractionDigits: 1 },
    usd: {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
      maximumFractionDigits: 6,
    },
    gbp: {
      style: 'currency',
      currency: 'GBP',
      minimumFractionDigits: 2,
      maximumFractionDigits: 6,
    },
  },
} as const

/** A fresh instance per call so tests do not share state. */
export function createAppI18n() {
  return createI18n({
    legacy: false,
    locale: LOCALE,
    fallbackLocale: LOCALE,
    messages: { [LOCALE]: tr },
    datetimeFormats,
    numberFormats,
  })
}

export const i18n = createAppI18n()
