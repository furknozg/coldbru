import { flushPromises, type DOMWrapper } from '@vue/test-utils'
import { vi } from 'vitest'

import * as analytics from '@/api/analytics/service'
import * as proposal from '@/api/proposal/service'
import {
  llmSeriesOf,
  overviewOf,
  overviewSeriesOf,
  promptVersionsOf,
  reportOf,
} from '@/features/dashboard/__tests__/fixtures'

/** Lazy views and charts load through dynamic imports; wait for them and for what they trigger. */
export async function settle() {
  for (let round = 0; round < 3; round++) {
    await vi.dynamicImportSettled()
    await flushPromises()
  }
}

/** Defaults for every Analytics and Proposal call the dashboard makes; a test overrides what it checks. */
export function mockDashboardApi() {
  vi.mocked(proposal.listCampaigns).mockResolvedValue([])
  vi.mocked(analytics.getCampaignSummary).mockResolvedValue(reportOf())
  vi.mocked(analytics.getOverview).mockResolvedValue(overviewOf())
  vi.mocked(analytics.getOverviewSeries).mockResolvedValue(overviewSeriesOf())
  vi.mocked(analytics.getLlmSeries).mockResolvedValue(llmSeriesOf())
  vi.mocked(analytics.getPromptVersionUsage).mockResolvedValue(promptVersionsOf())
}

/** "Today" for the date filter; only `Date` is faked so promises and timers keep running. */
export function freezeToday(now = new Date(2026, 9, 30, 15, 20)) {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(now)
}

/** A local calendar day as the RFC 3339 instant the API gets. */
export const utc = (year: number, month: number, day: number) =>
  new Date(year, month - 1, day).toISOString().replace(/\.000Z$/, 'Z')

/** The legend of the pie under `title`, as [label, value, share] rows. */
export function legend(
  root: { findAll: (selector: string) => DOMWrapper<Element>[] },
  title: string,
) {
  const figure = root.findAll('figure').find((f) => f.find('figcaption').text() === title)
  if (!figure) throw new Error(`no chart titled ${title}`)
  return figure.findAll('li').map((li) => li.findAll('span').map((span) => span.text()))
}
