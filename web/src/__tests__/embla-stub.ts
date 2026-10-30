import { onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'

type Handler = (api: FakeEmbla) => void

/**
 * Stands in for `embla-carousel` in every test: jsdom has no layout, so the real one cannot tell
 * how many snaps there are or scroll to one. This one counts the slides in the DOM (the children
 * of the container inside the viewport node), keeps the selected index and emits the events.
 */
export class FakeEmbla {
  private index = 0
  private readonly handlers = new Map<string, Set<Handler>>()

  constructor(private readonly viewport: HTMLElement) {}

  private count() {
    return this.viewport.firstElementChild?.children.length ?? 0
  }

  private emit(event: string) {
    for (const handler of this.handlers.get(event) ?? []) handler(this)
  }

  selectedScrollSnap() {
    return Math.min(this.index, Math.max(this.count() - 1, 0))
  }

  scrollSnapList() {
    return Array.from({ length: this.count() }, (_, i) => i)
  }

  canScrollPrev() {
    return this.selectedScrollSnap() > 0
  }

  canScrollNext() {
    return this.selectedScrollSnap() < this.count() - 1
  }

  scrollTo(index: number) {
    const next = Math.min(Math.max(index, 0), this.count() - 1)
    if (next === this.index) return
    this.index = next
    this.emit('select')
    // The real one settles when the animation ends.
    queueMicrotask(() => this.emit('settle'))
  }

  scrollPrev() {
    this.scrollTo(this.selectedScrollSnap() - 1)
  }

  scrollNext() {
    this.scrollTo(this.selectedScrollSnap() + 1)
  }

  reInit() {
    this.emit('reInit')
  }

  on(event: string, handler: Handler) {
    this.handlers.set(event, (this.handlers.get(event) ?? new Set()).add(handler))
    return this
  }

  off(event: string, handler: Handler) {
    this.handlers.get(event)?.delete(handler)
    return this
  }

  destroy() {
    this.handlers.clear()
  }
}

/** Same shape as `embla-carousel-vue`: a ref for the viewport node and a ref for the api. */
export default function useEmblaCarousel() {
  const node = ref<HTMLElement>()
  const api = shallowRef<FakeEmbla>()
  onMounted(() => {
    if (!node.value) return
    const created = new FakeEmbla(node.value)
    api.value = created
    // The real library announces `init` a tick after it is created.
    queueMicrotask(() => created.reInit())
  })
  onBeforeUnmount(() => api.value?.destroy())
  return [node, api] as const
}
