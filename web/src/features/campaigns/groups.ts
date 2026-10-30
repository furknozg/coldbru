import { CAMPAIGN_STATUSES, type CampaignInfo, type CampaignStatus } from './types'

export interface CampaignGroup {
  status: CampaignStatus
  items: readonly CampaignInfo[]
}

/**
 * The loaded campaigns split by status, in the order of `CAMPAIGN_STATUSES` (D-091); a status with
 * no campaign has no group. Inside a group the order of the list (newest first) is kept.
 */
export function groupByStatus(items: readonly CampaignInfo[]): CampaignGroup[] {
  return CAMPAIGN_STATUSES.map((status) => ({
    status,
    items: items.filter((c) => c.status === status),
  })).filter((group) => group.items.length > 0)
}
