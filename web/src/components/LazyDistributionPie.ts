import { defineAsyncComponent } from 'vue'

import ChartPlaceholder from './ChartPlaceholder.vue'

/**
 * `DistributionPie` with its own chunk: unovis is by far the largest dependency and only the
 * audience and plan steps draw a chart, so the rest of the app does not pay for it (D-092).
 */
export const LazyDistributionPie = defineAsyncComponent({
  loader: () => import('./DistributionPie.vue'),
  loadingComponent: ChartPlaceholder,
  errorComponent: ChartPlaceholder,
  delay: 150,
})
