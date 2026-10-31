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

type Root = { findAll: (selector: string) => DOMWrapper<Element>[] }

/** The title of every metric card under `root`, in order; a card without one gives ''. */
export const cardTitles = (root: Root) =>
  root.findAll('[data-slot="metric-card"]').map((card) => card.find('h3').text())

/** The metric card titled `title`. */
export function card(root: Root, title: string) {
  const found = root.findAll('[data-slot="metric-card"]').find((c) => c.find('h3').text() === title)
  if (!found) throw new Error(`no card titled ${title}`)
  return found
}

/**
 * The chart titled `title`: a pie by its caption, a time series or bar chart by its label (its
 * title is the heading of the card around it, not a caption of its own).
 */
export function chart(root: Root, title: string) {
  const figure = root.findAll('figure').find((f) => {
    const caption = f.find('figcaption')
    return (caption.exists() && caption.text() === title) || f.attributes('aria-label') === title
  })
  if (!figure) throw new Error(`no chart titled ${title}`)
  return figure
}

/** The legend of the pie under `title`, as [label, value, share] rows. */
export function legend(root: Root, title: string) {
  return chart(root, title)
    .findAll('li')
    .map((li) => li.findAll('span').map((span) => span.text()))
}
