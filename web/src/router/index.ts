import { createRouter, createWebHistory, type RouterHistory } from 'vue-router'

import CampaignPage from '@/views/CampaignPage.vue'
import HomeView from '@/views/HomeView.vue'

/** `/` is the empty state with the new-brief box; `/campaigns/:id` is one campaign (D-087). */
export const routes = [
  { path: '/', name: 'home', component: HomeView },
  { path: '/campaigns/:id', name: 'campaign', component: CampaignPage },
  { path: '/:rest(.*)*', redirect: { name: 'home' } },
]

/** The history is a parameter so tests can run the real routes on an in-memory history. */
export function createAppRouter(
  history: RouterHistory = createWebHistory(import.meta.env.BASE_URL),
) {
  return createRouter({ history, routes })
}

export default createAppRouter()
