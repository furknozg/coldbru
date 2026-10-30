import { computed, shallowRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import {
  activePreset,
  presetRange,
  rangeFromQuery,
  rangeToQuery,
  seriesQuery,
  toApiRange,
  type DateRange,
  type Preset,
} from './dateRange'

/**
 * The date filter of a dashboard page (D-097). The range lives in the URL (`?from=…&to=…`), so a
 * reload or a shared link shows the same days; the page reads it from here and never keeps a copy.
 * `now` is the moment the range was chosen: presets and the open end of a series are relative to it.
 */
export function useDateRange() {
  const route = useRoute()
  const router = useRouter()
  const now = shallowRef(new Date())

  const range = computed(() => rangeFromQuery(route.query))
  const preset = computed(() => activePreset(range.value, now.value))
  /** For the totals endpoints. */
  const apiRange = computed(() => toApiRange(range.value))
  /** For the series endpoints: both ends and a bucket size. */
  const series = computed(() => seriesQuery(range.value, now.value))

  function select(next: DateRange) {
    now.value = new Date()
    // `replace`: stepping through filters must not fill the history; other query keys stay.
    const query = { ...route.query }
    delete query.from
    delete query.to
    return router.replace({ query: { ...query, ...rangeToQuery(next) } })
  }

  const selectPreset = (name: Preset) => select(presetRange(name, new Date()))

  return { range, preset, apiRange, series, select, selectPreset }
}
