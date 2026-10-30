import {
  ChartColumnIcon,
  ChartNoAxesCombinedIcon,
  CpuIcon,
  LayoutDashboardIcon,
  RadioIcon,
} from '@lucide/vue'
import type { Component } from 'vue'

import type { CampaignStatus } from '@/features/campaigns/types'

import { ANALYTICS_STATUSES, LIVE_CAMPAIGN_STATUSES } from './live'

/**
 * The menu of the app as data (D-099). It is one list for every screen: "Genel bakış" is a link,
 * "Kampanyalar" is the campaign list (drawn by `CampaignList`, which owns its label), and
 * "Analitikler" is a group whose children are links, some of them accordions that list campaigns
 * (D-100). A new section is one more entry; T-044 adds
 * "Başarı metrikleri" to the children of "Analitikler" and nowhere else in the sidebar.
 */
export interface MenuLink {
  id: string
  /** Key of the label in `tr.json`. */
  label: string
  icon: Component
  /** Route name of the page. */
  route: string
  /** The page has the date filter, so the link carries the chosen range. */
  keepsRange?: boolean
  /**
   * The item is an accordion of the campaigns in these statuses; a row links to `route` with the
   * campaign id. `grouped` splits them by status, otherwise they are one list; `empty` is the
   * key of the line that says there is none.
   */
  campaigns?: { statuses: readonly CampaignStatus[]; grouped: boolean; empty: string }
}

export type MenuEntry =
  | ({ kind: 'link' } & MenuLink)
  | { kind: 'campaigns'; id: string }
  | { kind: 'group'; id: string; label: string; icon: Component; children: readonly MenuLink[] }

export const APP_MENU: readonly MenuEntry[] = [
  {
    kind: 'link',
    id: 'overview',
    label: 'dashboard.menu.overview',
    icon: LayoutDashboardIcon,
    route: 'dashboard',
    keepsRange: true,
  },
  { kind: 'campaigns', id: 'campaigns' },
  {
    kind: 'group',
    id: 'analytics',
    label: 'dashboard.menu.analytics',
    icon: ChartNoAxesCombinedIcon,
    children: [
      {
        id: 'campaign-analytics',
        label: 'dashboard.menu.campaigns',
        icon: ChartColumnIcon,
        route: 'analytics-campaigns',
        campaigns: {
          statuses: ANALYTICS_STATUSES,
          grouped: true,
          empty: 'dashboard.campaign.noneStarted',
        },
      },
      {
        id: 'llm',
        label: 'dashboard.menu.llm',
        icon: CpuIcon,
        route: 'analytics-llm',
        keepsRange: true,
      },
      {
        id: 'live',
        label: 'dashboard.menu.live',
        icon: RadioIcon,
        route: 'live',
        campaigns: {
          statuses: LIVE_CAMPAIGN_STATUSES,
          grouped: true,
          empty: 'dashboard.live.none',
        },
      },
    ],
  },
]

/** Every link of the menu, groups unfolded. */
export const MENU_LINKS: readonly MenuLink[] = APP_MENU.flatMap((entry) =>
  entry.kind === 'link' ? [entry] : entry.kind === 'group' ? entry.children : [],
)
