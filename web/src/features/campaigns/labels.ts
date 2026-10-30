import { useI18n } from 'vue-i18n'

import type { CampaignStatus, Channel, Slot, StepKind } from './types'

/** The two functions of vue-i18n the labels need; `i18n.global` and `useI18n()` both fit. */
export interface Translator {
  t: (key: string, named?: Record<string, unknown>) => string
  te: (key: string) => boolean
}

/** Display names of the enumerations the API sends; the strings live in `locales/tr.json`. */
export function createLabels({ t, te }: Translator) {
  const language = (code: string) =>
    te(`language.${code}`) ? t(`language.${code}`) : code.toUpperCase()
  return {
    stepKind: (kind: StepKind) => t(`step.kind.${kind}`),
    status: (status: CampaignStatus) => t(`campaign.status.${status}`),
    channel: (channel: Channel) => t(`channel.${channel}`),
    planMode: (mode: string) => t(`plan.mode.${mode}`),
    language,
    /** "Türkçe · SMS", "Türkçe · Europe/Istanbul": the parts of a slot that are set. */
    slot: (slot: Slot) =>
      [
        slot.language ? language(slot.language) : '',
        slot.channel ? t(`channel.${slot.channel}`) : '',
        slot.timezone ?? '',
      ]
        .filter(Boolean)
        .join(' · '),
  }
}

export type Labels = ReturnType<typeof createLabels>

export function useLabels(): Labels {
  const { t, te } = useI18n()
  return createLabels({ t, te })
}
