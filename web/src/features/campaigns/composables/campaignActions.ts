import { inject, provide, type InjectionKey } from 'vue'

import { useCampaignActions, type ActionHooks, type CampaignActions } from './useCampaignActions'

const campaignActionsKey: InjectionKey<CampaignActions> = Symbol('campaign-actions')

/**
 * Called once by the campaign page. The step panel, its variant cards and the composer all act on
 * the same campaign and share one `busy` flag, so they receive this instance instead of each
 * creating their own.
 */
export function provideCampaignActions(
  campaignId: () => string,
  hooks: ActionHooks,
): CampaignActions {
  const actions = useCampaignActions(campaignId, hooks)
  provide(campaignActionsKey, actions)
  return actions
}

export function useActions(): CampaignActions {
  const actions = inject(campaignActionsKey)
  if (!actions) throw new Error('useActions needs a campaign page above it')
  return actions
}
