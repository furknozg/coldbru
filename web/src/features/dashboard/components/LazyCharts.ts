import { defineAsyncComponent } from 'vue'

import ChartPlaceholder from '@/components/ChartPlaceholder.vue'

/**
 * The dashboard charts, each loaded on first use like `LazyDistributionPie` (D-092): unovis stays
 * out of the main bundle, and a chart that is loading or failed to load is one line of text.
 */
const options = { loadingComponent: ChartPlaceholder, errorComponent: ChartPlaceholder, delay: 150 }

export const LazyTimeSeriesChart = defineAsyncComponent({
  loader: () => import('./TimeSeriesChart.vue'),
  ...options,
})

export const LazyBarChart = defineAsyncComponent({
  loader: () => import('./BarChart.vue'),
  ...options,
})
