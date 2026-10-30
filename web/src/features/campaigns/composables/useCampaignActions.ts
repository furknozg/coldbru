import { computed, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'

import * as proposal from '@/api/proposal/service'

import { toProblem, type Problem } from '../problem'
import type { CampaignView, StepKind } from '../types'
import { useToasts } from './useToasts'

export type ActionResult = { ok: true } | { ok: false; problem: Problem }

export interface ActionHooks {
  /** Takes the view an action answered with. */
  applyView: (view: CampaignView) => void
  /** Fetches the view again, for actions that answer with something smaller. */
  refresh: () => Promise<void>
}

/**
 * Everything the marketer can do to one campaign (approve, reject, edit, regenerate, reopen, cancel,
 * confirm, add context, answer a question of the agent). One call runs at a time: `pending` names it and the buttons disable on
 * `busy`. A failed call becomes a toast, except the 422 of a call made `inline`, which goes back to
 * the form that sent it as `problem` so it can show the validation lines next to the field.
 * A 409 also refetches the view: the state the marketer saw has moved on.
 */
export function useCampaignActions(campaignId: () => string, hooks: ActionHooks) {
  const { t } = useI18n()
  const toasts = useToasts()
  const pending = shallowRef<string | null>(null)

  async function run(
    key: string,
    call: () => Promise<unknown>,
    options: { returnsView?: boolean; inline?: boolean } = {},
  ): Promise<ActionResult> {
    if (pending.value) return { ok: false, problem: toProblem(null, t) }
    pending.value = key
    try {
      const answer = await call()
      if (options.returnsView) hooks.applyView(answer as CampaignView)
      else await hooks.refresh()
      return { ok: true }
    } catch (cause) {
      const problem = toProblem(cause, t)
      if (problem.status === 409) void hooks.refresh()
      if (!(options.inline && problem.status === 422)) toasts.push(problem.message)
      return { ok: false, problem }
    } finally {
      pending.value = null
    }
  }

  return {
    pending: computed(() => pending.value),
    busy: computed(() => pending.value !== null),
    approve: (variantId: string) =>
      run(`approve:${variantId}`, () => proposal.approveVariant(variantId), { returnsView: true }),
    reject: (variantId: string, reason: string) =>
      run(`reject:${variantId}`, () => proposal.rejectVariant(variantId, reason), {
        returnsView: true,
        inline: true,
      }),
    edit: (variantId: string, content: Record<string, unknown>) =>
      run(`edit:${variantId}`, () => proposal.editVariant(variantId, content), { inline: true }),
    regenerateStep: (stepId: string) =>
      run(`regenerate:${stepId}`, () => proposal.regenerateStep(stepId), { returnsView: true }),
    regenerateCampaign: () =>
      run('regenerate-campaign', () => proposal.regenerateCampaign(campaignId()), {
        returnsView: true,
      }),
    reopen: (kind: StepKind) =>
      run(`reopen:${kind}`, () => proposal.reopenStep(campaignId(), kind)),
    cancel: (reason?: string) =>
      run('cancel', () => proposal.cancelCampaign(campaignId(), reason), {
        returnsView: true,
        inline: true,
      }),
    confirm: () =>
      run('confirm', () => proposal.confirmCampaign(campaignId()), { returnsView: true }),
    addContext: (body: string) =>
      run('context', () => proposal.addContext(campaignId(), body), { inline: true }),
    answer: (questionId: string, answer: string) =>
      run(`answer:${questionId}`, () => proposal.answerQuestion(questionId, answer), {
        returnsView: true,
        inline: true,
      }),
  }
}

export type CampaignActions = ReturnType<typeof useCampaignActions>
