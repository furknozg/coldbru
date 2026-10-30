import { computed, onBeforeUnmount, shallowRef, watch } from 'vue'

import type { CarouselApi } from '@/components/ui/carousel'

import { nextPendingIndex } from '../carousel'
import type { VariantView } from '../types'

/**
 * Which slide of the variant carousel is shown, and the move to the next slide that waits for an
 * approval once the one in view is approved (D-093). The slides are the variants in the order the
 * step lists them; embla owns the scrolling and tells which snap is selected.
 */
export function useVariantCarousel(slides: () => readonly VariantView[]) {
  const api = shallowRef<CarouselApi>()
  const selected = shallowRef(0)

  const total = computed(() => slides().length)
  /** Slides come and go while the step is open; the shown index never points past the last one. */
  const current = computed(() => Math.min(selected.value, Math.max(total.value - 1, 0)))

  /**
   * True from the start of a swipe or a scroll until the carousel settles: then the neighbours are
   * on screen and keep their height. Once settled, only the shown slide takes up room, so a short
   * SMS card is not followed by the blank height of a tall email.
   */
  const moving = shallowRef(false)
  let changed = false

  const sync = () => {
    selected.value = api.value?.selectedScrollSnap() ?? 0
  }
  const onSelect = () => {
    sync()
    changed = true
    moving.value = true
  }
  const onPointerDown = () => {
    changed = false
    moving.value = true
  }
  // A press that moved nothing leaves no animation to wait for.
  const onPointerUp = () => {
    if (!changed) moving.value = false
  }
  const onSettle = () => {
    moving.value = false
  }

  const listeners = [
    ['select', onSelect],
    ['reInit', sync],
    ['pointerDown', onPointerDown],
    ['pointerUp', onPointerUp],
    ['settle', onSettle],
  ] as const

  function detach() {
    for (const [event, handler] of listeners) api.value?.off(event, handler)
  }

  function bind(next: CarouselApi) {
    detach()
    api.value = next
    if (!next) return
    for (const [event, handler] of listeners) next.on(event, handler)
    sync()
  }

  onBeforeUnmount(detach)

  // A variant that turns approved while it is the one in view sends the reader on to the next
  // slide that still waits; one approved while the reader looks at another slide changes nothing.
  watch(
    () => slides().flatMap((v) => (v.status === 'approved' ? [v.id] : [])),
    (_now, before) => {
      const list = slides()
      const approvedNow = list[current.value]
      if (!approvedNow || approvedNow.status !== 'approved' || before.includes(approvedNow.id))
        return
      const target = nextPendingIndex(list, current.value)
      if (target >= 0) api.value?.scrollTo(target)
    },
    { flush: 'post' },
  )

  return { bind, current, total, moving }
}
