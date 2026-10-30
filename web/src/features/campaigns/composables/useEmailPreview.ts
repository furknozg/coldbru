import { onScopeDispose, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { previewVariant } from '@/api/proposal/service'

import { toProblem, type Problem } from '../problem'
import type { EmailPreview } from '../types'

/** How long the edit form waits after a keystroke before it asks for a new preview (D-090). */
export const PREVIEW_DEBOUNCE_MS = 400

/**
 * The rendered preview of an email variant (`POST /variants/{id}/preview`, D-090). It is fetched
 * once on creation; while `content` returns unsaved edited blocks, every change schedules a new
 * request after `debounceMs`, and only the newest answer is kept. A failed request keeps the last
 * preview on screen and reports why in `problem` (a 422 lists the validation lines).
 */
export function useEmailPreview(
  variantId: () => string,
  content: () => Record<string, unknown> | undefined,
  debounceMs = PREVIEW_DEBOUNCE_MS,
) {
  const { t } = useI18n()
  const preview = shallowRef<EmailPreview | null>(null)
  const loading = shallowRef(false)
  const problem = shallowRef<Problem | null>(null)

  let generation = 0
  let timer: ReturnType<typeof setTimeout> | undefined

  async function refresh() {
    const mine = ++generation
    loading.value = true
    try {
      const next = await previewVariant(variantId(), content())
      if (mine !== generation) return
      preview.value = next
      problem.value = null
    } catch (cause) {
      if (mine !== generation) return
      problem.value = toProblem(cause, t)
    } finally {
      if (mine === generation) loading.value = false
    }
  }

  function schedule() {
    clearTimeout(timer)
    timer = setTimeout(() => void refresh(), debounceMs)
  }

  // `content` is a fresh object whenever a field changes, so the watcher fires per change.
  watch(content, schedule)
  void refresh()

  onScopeDispose(() => {
    clearTimeout(timer)
    generation++
  })

  return { preview, loading, problem, refresh }
}
