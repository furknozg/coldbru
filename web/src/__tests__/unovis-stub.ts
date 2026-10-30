import { defineComponent, h } from 'vue'

/**
 * Stands in for `@unovis/vue` in every test: jsdom has no layout, so the real charts cannot draw.
 * The containers report how many arcs or rows they were given; the XY marks report their series
 * and the values the accessors return for every row, so a test can read what would be drawn.
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

export const VisXYContainer = defineComponent({
  name: 'VisXYContainer',
  props: { data: { type: Array, default: () => [] } },
  setup:
    (props, { slots }) =>
    () =>
      h(
        'div',
        { 'data-testid': 'xy-container', 'data-rows': props.data.length },
        slots.default?.(),
      ),
})

type Accessor = (row: unknown) => number

/** A mark that draws `y` (one accessor or one per series); `data-y` is every series' values. */
const xyMark = (name: string, testid: string) =>
  defineComponent({
    name,
    props: {
      y: { type: [Function, Array], default: undefined },
      color: { type: [String, Array], default: undefined },
    },
    setup(props) {
      return () => {
        const accessors = (Array.isArray(props.y) ? props.y : [props.y]) as Accessor[]
        return h('i', {
          'data-testid': testid,
          'data-series': accessors.length,
          'data-colors': Array.isArray(props.color) ? props.color.join('|') : props.color,
        })
      }
    },
  })

export const VisLine = xyMark('VisLine', 'line')
export const VisArea = xyMark('VisArea', 'area')
export const VisGroupedBar = xyMark('VisGroupedBar', 'bars')
export const VisAxis = defineComponent({
  name: 'VisAxis',
  props: { type: { type: String, default: 'x' } },
  setup: (props) => () => h('i', { 'data-testid': `axis-${props.type}` }),
})

export const VisTooltip = defineComponent({ name: 'VisTooltip', setup: () => () => null })
export const VisCrosshair = VisTooltip
