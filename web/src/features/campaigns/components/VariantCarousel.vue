<script setup lang="ts">
import { ChevronLeftIcon, ChevronRightIcon } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  Carousel,
  CarouselContent,
  CarouselItem,
  CarouselNext,
  CarouselPrevious,
} from '@/components/ui/carousel'

import { orderSlides } from '../carousel'
import { useVariantCarousel } from '../composables/useVariantCarousel'
import type { StepKind, VariantView } from '../types'
import PendingBadge from './PendingBadge.vue'
import VariantCard from './VariantCard.vue'

/**
 * The variants of a step (or of one channel of it), one at a time in a horizontal carousel
 * (D-095): arrows, keyboard arrows and swipe move between them, the position reads "3 / 6", and a
 * round badge at the corner of the card area counts the slots that still wait for an approval.
 * Slides are ordered by language, then time zone. Each slide is the variant's own card; the
 * carousel adds no box of its own around it.
 */
const props = defineProps<{
  variants: readonly VariantView[]
  kind: StepKind
  /** The step is open in a campaign that is still drafting: draft slides carry their actions. */
  actionable: boolean
  /** Slots without an approved variant; the badge is hidden at 0. */
  pending: number
  /** Names the carousel when a step has several (the channel); shown in front of the arrows. */
  label?: string
}>()

const { t } = useI18n()
const regionLabel = computed(() =>
  props.label ? t('carousel.group', { name: props.label }) : t('carousel.label'),
)
const slides = computed(() => orderSlides(props.variants))
const { bind, current, total, moving } = useVariantCarousel(() => slides.value)

/** Arrow keys in a text box move the caret; they must not reach the carousel's key handler. */
function keepTyping(event: KeyboardEvent) {
  const target = event.target
  if (target instanceof HTMLElement && target.matches('input, textarea, select, [contenteditable]'))
    event.stopPropagation()
}
</script>

<template>
  <Carousel
    v-if="total > 0"
    class="flex flex-col gap-2"
    :aria-label="regionLabel"
    :opts="{ loop: false }"
    @init-api="bind"
  >
    <div v-if="label || total > 1" class="flex items-center gap-2">
      <span v-if="label" class="text-xs font-medium" data-carousel-label>{{ label }}</span>
      <template v-if="total > 1">
        <CarouselPrevious class="static inset-auto my-0">
          <ChevronLeftIcon />
          <span class="sr-only">{{ t('carousel.previous') }}</span>
        </CarouselPrevious>
        <span class="text-muted-foreground text-xs tabular-nums" aria-live="polite" data-position>
          {{ t('carousel.position', { current: current + 1, total }) }}
        </span>
        <CarouselNext class="static inset-auto my-0">
          <ChevronRightIcon />
          <span class="sr-only">{{ t('carousel.next') }}</span>
        </CarouselNext>
      </template>
    </div>

    <div class="relative">
      <CarouselContent class="items-start" @keydown="keepTyping">
        <CarouselItem
          v-for="(variant, index) in slides"
          :key="variant.id"
          :aria-label="t('carousel.slide', { current: index + 1, total })"
          :class="index !== current && !moving && 'h-0 overflow-hidden'"
          :inert="index === current ? undefined : true"
          :data-current="index === current ? '' : undefined"
        >
          <VariantCard
            :variant="variant"
            :kind="kind"
            :actionable="actionable && variant.status === 'draft'"
          />
        </CarouselItem>
      </CarouselContent>
      <PendingBadge :count="pending" :label="label" />
    </div>
  </Carousel>

  <!-- Slots wait but there is no draft to show: the label, the count and one line, no slides. -->
  <div v-else class="flex flex-col gap-2" role="group" :aria-label="regionLabel">
    <span v-if="label" class="text-xs font-medium" data-carousel-label>{{ label }}</span>
    <div class="relative">
      <p class="text-muted-foreground pr-5 text-sm" data-empty>{{ t('carousel.empty') }}</p>
      <PendingBadge :count="pending" :label="label" />
    </div>
  </div>
</template>
