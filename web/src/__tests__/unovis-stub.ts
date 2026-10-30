import { defineComponent, h } from 'vue'

/**
 * Stands in for `@unovis/vue` in every test: jsdom has no layout, so the real charts cannot draw.
 * The container reports how many arcs it was given.
 */
export const VisSingleContainer = defineComponent({
  name: 'VisSingleContainer',
  props: { data: { type: Array, default: () => [] } },
  setup:
    (props, { slots }) =>
    () =>
      h('div', { 'data-testid': 'container', 'data-arcs': props.data.length }, slots.default?.()),
})

export const VisDonut = defineComponent({
  name: 'VisDonut',
  setup: () => () => h('i', { 'data-testid': 'donut' }),
})

export const VisTooltip = defineComponent({ name: 'VisTooltip', setup: () => () => null })
export const VisCrosshair = VisTooltip
