import { inject, provide, type InjectionKey } from 'vue'

import { useCampaigns, type CampaignList } from './useCampaigns'

const campaignListKey: InjectionKey<CampaignList> = Symbol('campaign-list')

/** Called once, by the shell: the list lives as long as the window and every page shares it. */
export function provideCampaignList(): CampaignList {
  const list = useCampaigns()
  provide(campaignListKey, list)
  return list
}

/** For pages that report a campaign they created or changed so the list shows it at once. */
export function useCampaignList(): CampaignList {
  const list = inject(campaignListKey)
  if (!list) throw new Error('useCampaignList needs a provider: wrap the page in the app shell')
  return list
}
