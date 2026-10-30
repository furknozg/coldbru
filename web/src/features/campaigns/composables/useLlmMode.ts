import { onMounted, shallowRef } from 'vue'

import { getLlmMode } from '@/api/proposal/service'

/** The LLM mode the Proposal service runs in (`anthropic` or `mock`, D-072), asked once. */
export function useLlmMode() {
  const mode = shallowRef<string | null>(null)
  onMounted(async () => {
    mode.value = await getLlmMode()
  })
  return { mode }
}
