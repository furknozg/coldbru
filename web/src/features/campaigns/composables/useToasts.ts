import { readonly, shallowRef } from 'vue'

export interface Toast {
  id: number
  tone: 'error' | 'info'
  message: string
}

const TOAST_MS = 8000

// Transient notices belong to the window, not to a campaign, so one list serves every caller. It
// holds no server state (D-012).
const list = shallowRef<readonly Toast[]>([])
let nextId = 1

function dismiss(id: number) {
  list.value = list.value.filter((toast) => toast.id !== id)
}

function push(message: string, tone: Toast['tone'] = 'error') {
  const id = nextId++
  list.value = [...list.value, { id, tone, message }]
  setTimeout(() => dismiss(id), TOAST_MS)
  return id
}

export function useToasts() {
  return { toasts: readonly(list), push, dismiss }
}

/** Test helper: empties the list between tests. */
export function resetToasts() {
  list.value = []
}
