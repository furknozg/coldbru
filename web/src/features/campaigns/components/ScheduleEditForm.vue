<script setup lang="ts">
import { now, today, type DateValue } from '@internationalized/date'
import { computed, shallowRef, useId } from 'vue'
import { useI18n } from 'vue-i18n'

import InlineProblems from '@/components/InlineProblems.vue'
import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import { Input } from '@/components/ui/input'

import { scheduleContent } from '../content'
import type { Problem } from '../problem'
import { dateOf, rfc3339, timeOf, zonedFrom, zonedOf } from '../schedule'
import type { VariantView } from '../types'

/**
 * Edits the send time of one (language, time zone) slot in the zone of that slot (D-119): the day
 * comes from a calendar, the time from an HH:mm field, and "send immediately" sends `send_at: null`.
 * The server receives RFC 3339 with the offset the zone has at that instant and checks it; its
 * problems show under the fields. The browser-local equivalent is shown next to the slot's time so
 * the marketer can place it in their own day. The rationale is kept as it is.
 */
const props = defineProps<{ variant: VariantView; busy: boolean; problem: Problem | null }>()

const emit = defineEmits<{
  save: [content: { send_at: string | null; rationale: string }]
  cancel: []
}>()

const { t, d } = useI18n()
const id = useId()

const timeZone = props.variant.timezone || 'UTC'
const initial = scheduleContent(props.variant)
const stored = initial.sendAt ? zonedOf(initial.sendAt, timeZone) : null
/** A slot that sends immediately opens the fields on the next full hour, so a switch has a start. */
const fallback = now(timeZone).add({ hours: 1 }).set({ minute: 0, second: 0, millisecond: 0 })

const immediately = shallowRef(stored === null)
const date = shallowRef<DateValue | undefined>(dateOf(stored ?? fallback))
const time = shallowRef(timeOf(stored ?? fallback))

/** The server accepts now + 5 minutes to now + 30 days (`flow.MinSendDelay`, `flow.MaxSendDelay`). */
const earliestDay = today(timeZone)
const latestDay = earliestDay.add({ days: 30 })

const zoned = computed(() =>
  date.value ? zonedFrom(dateOf(date.value), time.value, timeZone) : null,
)
const complete = computed(() => immediately.value || zoned.value !== null)

const inZone = computed(() =>
  zoned.value ? d(zoned.value.toDate(), { key: 'zoned', timeZone }) : '',
)
const local = computed(() => (zoned.value ? d(zoned.value.toDate(), 'zoned') : ''))

function save() {
  if (!complete.value) return
  emit('save', {
    send_at: immediately.value ? null : rfc3339(zoned.value!),
    rationale: initial.rationale,
  })
}
</script>

<template>
  <form class="flex flex-col gap-3" :aria-label="t('schedule.edit.title')" @submit.prevent="save">
    <label class="flex items-center gap-2 text-sm font-medium">
      <input v-model="immediately" type="checkbox" name="immediately" class="accent-primary" />
      {{ t('schedule.immediately') }}
    </label>

    <div v-if="!immediately" class="grid gap-3 sm:grid-cols-[auto_1fr]">
      <Calendar
        :model-value="date"
        :min-value="earliestDay"
        :max-value="latestDay"
        locale="tr-TR"
        :week-starts-on="1"
        :aria-label="t('schedule.edit.date')"
        class="rounded-lg border"
        @update:model-value="date = $event"
      />
      <div class="flex min-w-0 flex-col gap-3">
        <div class="flex flex-col gap-1">
          <label :for="`${id}-time`" class="text-sm font-medium">
            {{ t('schedule.edit.time', { zone: timeZone }) }}
          </label>
          <Input :id="`${id}-time`" v-model="time" type="time" name="time" class="w-32" />
        </div>
        <template v-if="zoned">
          <p class="text-sm" data-in-zone>
            <span class="text-muted-foreground">{{ t('schedule.sendAt') }}: </span>
            <span class="font-medium">{{ inZone }}</span>
          </p>
          <p class="text-muted-foreground text-xs" data-local>
            {{ t('schedule.edit.localEquivalent', { time: local }) }}
          </p>
        </template>
        <p v-else class="text-muted-foreground text-xs" role="status">
          {{ t('schedule.edit.incomplete') }}
        </p>
      </div>
    </div>

    <InlineProblems v-if="problem" :problem="problem" />
    <div class="flex justify-end gap-2">
      <Button type="button" variant="outline" :disabled="busy" @click="emit('cancel')">
        {{ t('common.cancel') }}
      </Button>
      <Button type="submit" :disabled="busy || !complete">
        {{ busy ? t('common.sending') : t('common.save') }}
      </Button>
    </div>
  </form>
</template>
