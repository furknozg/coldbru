import type { Channel, VariantView } from './types'

/**
 * The shapes of `VariantView.content` per step kind. The API publishes the field as an opaque
 * object (D-034), so each parser reads what it needs defensively: a variant stored by an older
 * build can lack a field, and the panel must still draw.
 */

export interface Coverage {
  both: number
  email_only: number
  sms_only: number
  unreachable: number
}

export interface LocaleAudience {
  language: string
  timezone?: string
  count: number
  coverage: Coverage
}

export interface AudienceContent {
  dsl: string
  count: number
  coverage: Coverage
  byLanguage: LocaleAudience[]
  byLocale: LocaleAudience[]
}

export type PlanMode = 'single' | 'primary_fallback' | 'both'
export const PLAN_MODES: readonly PlanMode[] = ['single', 'primary_fallback', 'both']
export const CHANNELS: readonly Channel[] = ['email', 'sms']

export interface PlanContent {
  mode: PlanMode
  primary: Channel
  rationale: string
  recipients: { email: number; sms: number; total: number }
  byLanguage: { language: string; email: number; sms: number }[]
}

export interface SmsContent {
  text: string
  encoding: string
  segments: number
  renderedLength: number
  cost: { amount: string; currency: 'USD' | 'GBP' } | null
}

/** The paragraph count the content validators accept (internal/content limits, D-071). */
export const PARAGRAPHS_MIN = 1
export const PARAGRAPHS_MAX = 4

/** The blocks of an email as the edit and preview endpoints take them (D-090). */
export type EmailEdit = {
  subject: string
  preheader: string
  headline: string
  paragraphs: string[]
  cta_label: string
}

export interface EmailContent {
  subject: string
  preheader: string
  headline: string
  paragraphs: string[]
  ctaLabel: string
  cost: { amount: string; currency: 'USD' | 'GBP' } | null
}

export interface ScheduleContent {
  sendAt: string | null
  rationale: string
}

type Raw = Record<string, unknown>

const raw = (variant: VariantView): Raw => (variant.content ?? {}) as Raw
const str = (value: unknown, fallback = ''): string =>
  typeof value === 'string' ? value : fallback
const num = (value: unknown, fallback = 0): number =>
  typeof value === 'number' && Number.isFinite(value) ? value : fallback
const list = (value: unknown): unknown[] => (Array.isArray(value) ? value : [])

function coverage(value: unknown): Coverage {
  const c = (value ?? {}) as Raw
  return {
    both: num(c.both),
    email_only: num(c.email_only),
    sms_only: num(c.sms_only),
    unreachable: num(c.unreachable),
  }
}

function locales(value: unknown): LocaleAudience[] {
  return list(value).map((entry) => {
    const e = (entry ?? {}) as Raw
    return {
      language: str(e.language),
      timezone: typeof e.timezone === 'string' ? e.timezone : undefined,
      count: num(e.count),
      coverage: coverage(e.coverage),
    }
  })
}

export function audienceContent(variant: VariantView): AudienceContent {
  const c = raw(variant)
  return {
    dsl: str(c.dsl),
    count: num(c.count),
    coverage: coverage(c.coverage),
    byLanguage: locales(c.by_language),
    byLocale: locales(c.by_locale),
  }
}

export function planContent(variant: VariantView): PlanContent {
  const c = raw(variant)
  const recipients = (c.recipients ?? {}) as Raw
  return {
    mode: (PLAN_MODES as readonly string[]).includes(str(c.mode)) ? (c.mode as PlanMode) : 'single',
    primary: c.primary === 'sms' ? 'sms' : 'email',
    rationale: str(c.rationale),
    recipients: {
      email: num(recipients.email),
      sms: num(recipients.sms),
      total: num(recipients.total),
    },
    byLanguage: list(c.by_language).map((entry) => {
      const e = (entry ?? {}) as Raw
      return { language: str(e.language), email: num(e.email), sms: num(e.sms) }
    }),
  }
}

/** USD since D-083; variants stored before it carry `estimated_cost_gbp`. */
function costOf(c: Raw): { amount: string; currency: 'USD' | 'GBP' } | null {
  if (typeof c.estimated_cost_usd === 'string')
    return { amount: c.estimated_cost_usd, currency: 'USD' }
  if (typeof c.estimated_cost_gbp === 'string')
    return { amount: c.estimated_cost_gbp, currency: 'GBP' }
  return null
}

export function smsContent(variant: VariantView): SmsContent {
  const c = raw(variant)
  return {
    text: str(c.text),
    encoding: str(c.encoding),
    segments: num(c.segments),
    renderedLength: num(c.rendered_length),
    cost: costOf(c),
  }
}

export function emailContent(variant: VariantView): EmailContent {
  const c = raw(variant)
  return {
    subject: str(c.subject),
    preheader: str(c.preheader),
    headline: str(c.headline),
    paragraphs: list(c.paragraphs).map((p) => str(p)),
    ctaLabel: str(c.cta_label),
    cost: costOf(c),
  }
}

export function scheduleContent(variant: VariantView): ScheduleContent {
  const c = raw(variant)
  return { sendAt: typeof c.send_at === 'string' ? c.send_at : null, rationale: str(c.rationale) }
}
