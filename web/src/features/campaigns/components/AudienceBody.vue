<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { LazyDistributionPie } from '@/components/LazyDistributionPie'

import { audienceContent } from '../content'
import {
  bySize,
  CHANNEL_COLORS,
  coverageSlices,
  foldSmallest,
  languageTotals,
  SERIES_COLORS,
} from '../distribution'
import { useLabels } from '../labels'
import type { VariantView } from '../types'

/** The audience proposal: the rule, how many people it reaches and on which channels (D-010). */
const props = defineProps<{ variant: VariantView }>()

const { t, n } = useI18n()
const labels = useLabels()

const audience = computed(() => audienceContent(props.variant))

const MAX_SLICES = SERIES_COLORS.length

const other = computed(() => ({ key: 'other', label: t('chart.other'), value: 0 }))

const coverage = computed(() =>
  coverageSlices(audience.value.coverage, (kind) => t(`audience.coverageKind.${kind}`)).map(
    (slice) => ({ ...slice, color: CHANNEL_COLORS[slice.key] }),
  ),
)

const languages = computed(() =>
  foldSmallest(
    bySize(
      languageTotals(audience.value).map(({ language, count }) => ({
        key: language,
        label: labels.language(language),
        value: count,
      })),
    ),
    MAX_SLICES,
    other.value,
  ),
)

// Language + time zone exists from D-076 on; an older variant has no zones to chart.
const locales = computed(() =>
  foldSmallest(
    bySize(
      audience.value.byLocale.map(({ language, timezone, count }) => ({
        key: `${language}|${timezone ?? ''}`,
        label: labels.slot({ language, timezone }),
        value: count,
      })),
    ),
    MAX_SLICES,
    other.value,
  ),
)
</script>

<template>
  <div class="flex flex-col gap-3 text-sm">
    <pre
      class="border-caramel/40 max-h-40 overflow-auto border-l-2 pl-2 font-mono text-xs break-all whitespace-pre-wrap"
      :aria-label="t('audience.dsl')"
      >{{ audience.dsl }}</pre>
    <p>
      <span class="text-lg font-semibold tabular-nums">{{ n(audience.count, 'int') }}</span>
      <span class="text-muted-foreground"> {{ t('audience.people') }}</span>
    </p>

    <div class="flex flex-wrap gap-x-8 gap-y-5">
      <LazyDistributionPie
        class="min-w-56 flex-1 basis-56"
        :title="t('audience.coverage')"
        :slices="coverage"
      />
      <LazyDistributionPie
        class="min-w-56 flex-1 basis-56"
        :title="t('audience.language')"
        :slices="languages"
      />
      <LazyDistributionPie
        v-if="locales.length"
        class="min-w-56 flex-1 basis-56"
        :title="t('audience.locale')"
        :slices="locales"
      />
    </div>
  </div>
</template>
