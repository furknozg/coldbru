import { createRouter, createWebHistory, type RouteRecordRaw, type RouterHistory } from 'vue-router'

import CampaignPage from '@/views/CampaignPage.vue'
import HomeView from '@/views/HomeView.vue'

declare module 'vue-router' {
  interface RouteMeta {
    /** Which left column the shell draws; a route without it is on the campaign screen. */
    screen?: 'dashboard'
  }
}

/**
 * `/` is the dashboard (D-097): the overview, with the other sections as child routes. A new
 * campaign is written at `/campaigns/new` (the brief box) and one campaign is `/campaigns/:id`.
 * The dashboard pages are loaded when first visited, so the chat does not carry their code.
 * Dashboard sections live under `/analytics` and `/live` because `/campaigns/*` is the chat's.
 */
export const routes: RouteRecordRaw[] = [
  {
    path: '/',
    meta: { screen: 'dashboard' },
    children: [
      {
        path: '',
        name: 'dashboard',
        component: () => import('@/views/dashboard/OverviewView.vue'),
      },
      {
        path: 'analytics/campaigns/:id?',
        name: 'analytics-campaigns',
        component: () => import('@/views/dashboard/CampaignAnalyticsView.vue'),
      },
      {
        path: 'analytics/llm',
        name: 'analytics-llm',
        component: () => import('@/views/dashboard/LlmAnalyticsView.vue'),
      },
      {
        path: 'live/:id?',
        name: 'live',
        component: () => import('@/views/dashboard/LiveView.vue'),
      },
    ],
  },
  { path: '/campaigns/new', name: 'new-campaign', component: HomeView },
  { path: '/campaigns/:id', name: 'campaign', component: CampaignPage },
  { path: '/:rest(.*)*', redirect: { name: 'dashboard' } },
]

/** The history is a parameter so tests can run the real routes on an in-memory history. */
export function createAppRouter(
  history: RouterHistory = createWebHistory(import.meta.env.BASE_URL),
) {
  return createRouter({ history, routes })
}

export default createAppRouter()
