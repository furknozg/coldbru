<script setup lang="ts">
import { CalendarDate, parseDate as parseCalendarDate } from '@internationalized/date'
import { CalendarIcon } from '@lucide/vue'
import type { DateRange as CalendarRange } from 'reka-ui'
import { computed, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { RangeCalendar } from '@/components/ui/range-calendar'

import { PRESETS, formatDate, parseDate, type DateRange, type Preset } from '../dateRange'

/**
 * The date filter (D-097): five presets and a custom range from a calendar. It only shows and
 * reports; the page owns the range (it lives in the URL). A range is complete once both days are
 * picked, then the calendar closes.
 */
const props = defineProps<{ range: DateRange; preset: Preset | null }>()
const emit = defineEmits<{ preset: [Preset]; range: [DateRange] }>()

const { t, d } = useI18n()

const open = shallowRef(false)
const custom = computed(() => props.preset === null)

const picked = computed<CalendarRange>(() => ({
  start: props.range.from ? parseCalendarDate(props.range.from) : undefined,
  end: props.range.to ? parseCalendarDate(props.range.to) : undefined,
}))

const today = computed(() => {
  const now = new Date()
  return new CalendarDate(now.getFullYear(), now.getMonth() + 1, now.getDate())
})

const customLabel = computed(() => {
  const { from, to } = props.range
  if (!custom.value || (!from && !to)) return t('dashboard.range.custom')
  const show = (day: string | null) => {
    const date = day ? parseDate(day) : null
    return date ? d(date, 'date') : '…'
  }
  return `${show(from)} – ${show(to)}`
})

function onPick(value: CalendarRange) {
  if (!value.start || !value.end) return
  emit('range', { from: formatDate(toDate(value.start)), to: formatDate(toDate(value.end)) })
  open.value = false
}

const toDate = (day: { year: number; month: number; day: number }) =>
  new Date(day.year, day.month - 1, day.day)
</script>

<template>
  <div
    class="flex flex-wrap items-center gap-1.5"
    role="group"
    :aria-label="t('dashboard.range.label')"
  >
    <Button
      v-for="name in PRESETS"
      :key="name"
      type="button"
      size="sm"
      variant="ghost"
      :class="preset === name ? 'bg-accent text-accent-foreground' : 'text-muted-foreground'"
      :aria-pressed="preset === name"
      @click="emit('preset', name)"
    >
      {{ t(`dashboard.range.${name}`) }}
    </Button>
    <Popover v-model:open="open">
      <PopoverTrigger as-child>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          :class="
            custom && (range.from || range.to)
              ? 'bg-accent text-accent-foreground'
              : 'text-muted-foreground'
          "
          :aria-pressed="custom && Boolean(range.from || range.to)"
        >
          <CalendarIcon aria-hidden="true" />
          {{ customLabel }}
        </Button>
      </PopoverTrigger>
      <PopoverContent class="w-auto p-0" align="end">
        <RangeCalendar
          :model-value="picked"
          :max-value="today"
          locale="tr-TR"
          :week-starts-on="1"
          :number-of-months="1"
          :aria-label="t('dashboard.range.calendar')"
          @update:model-value="onPick"
        />
      </PopoverContent>
    </Popover>
  </div>
</template>
