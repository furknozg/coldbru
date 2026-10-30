import { shallowRef, toValue, watch, type MaybeRefOrGetter } from 'vue'

/**
 * One request that follows a key: it runs when the key changes (and at once), and only the answer
 * to the latest key is kept. A `null` key means there is nothing to fetch. The previous data stays
 * while the next is on its way, so a changed filter does not blank the page; a failure clears it.
 */
export function useResource<K, T>(key: MaybeRefOrGetter<K | null>, fetch: (key: K) => Promise<T>) {
  const data = shallowRef<T | null>(null)
  const loading = shallowRef(false)
  const error = shallowRef<unknown>(null)
  let generation = 0

  async function run(target: K | null) {
    const mine = ++generation
    error.value = null
    if (target === null) {
      data.value = null
      loading.value = false
      return
    }
    loading.value = true
    try {
      const next = await fetch(target)
      if (mine === generation) data.value = next
    } catch (cause) {
      if (mine === generation) {
        data.value = null
        error.value = cause
      }
    } finally {
      if (mine === generation) loading.value = false
    }
  }

  watch(() => toValue(key), run, { immediate: true })

  return { data, loading, error, reload: () => run(toValue(key)) }
}
