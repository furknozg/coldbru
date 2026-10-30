import { afterEach, vi } from 'vitest'

// The charts cannot draw in jsdom (no layout); every test gets the stub instead.
vi.mock('@unovis/vue', () => import('./unovis-stub'))

/**
 * jsdom has no `matchMedia`. The default is a desktop-wide window; a test for the drawer layout
 * calls `setViewportWide(false)` before it mounts anything.
 */
export function setViewportWide(wide: boolean) {
  vi.stubGlobal(
    'matchMedia',
    (query: string): MediaQueryList =>
      ({
        matches: wide,
        media: query,
        onchange: null,
        addEventListener: () => {},
        removeEventListener: () => {},
        addListener: () => {},
        removeListener: () => {},
        dispatchEvent: () => false,
      }) as MediaQueryList,
  )
}

setViewportWide(true)

// reka-ui measures with ResizeObserver, which jsdom lacks; scrollIntoView is missing too.
class NoopResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
vi.stubGlobal('ResizeObserver', NoopResizeObserver)
// The API tests run in a plain Node environment, where there is no DOM to patch.
if (typeof Element !== 'undefined') Element.prototype.scrollIntoView = () => {}

afterEach(() => {
  // `unstubAllGlobals` in a test would remove the stubs above; put the defaults back.
  setViewportWide(true)
  vi.stubGlobal('ResizeObserver', NoopResizeObserver)
})
