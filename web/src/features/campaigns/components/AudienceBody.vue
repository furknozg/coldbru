<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { audienceContent } from '../content'
import { useLabels } from '../labels'
import type { VariantView } from '../types'

/** The audience proposal: the rule, how many people it reaches and on which channels (D-010). */
const props = defineProps<{ variant: VariantView }>()

const { t, n } = useI18n()
const labels = useLabels()

const audience = computed(() => audienceContent(props.variant))

// by_locale (language + time zone, D-076) when the variant has it, else the older by_language.
const breakdown = computed(() =>
  audience.value.byLocale.length ? audience.value.byLocale : audience.value.byLanguage,
)
const hasZone = computed(() => breakdown.value.some((row) => row.timezone))

const coverageRows = computed(() => [
  { key: 'both', value: audience.value.coverage.both },
  { key: 'email_only', value: audience.value.coverage.email_only },
  { key: 'sms_only', value: audience.value.coverage.sms_only },
  { key: 'unreachable', value: audience.value.coverage.unreachable },
])
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

    <table class="w-full text-left text-xs">
      <caption class="text-muted-foreground pb-1 text-left font-medium">
        {{
          t('audience.coverage')
        }}
      </caption>
      <tbody>
        <tr v-for="row in coverageRows" :key="row.key" class="border-t">
          <th scope="row" class="py-1 pr-2 font-normal">
            {{ t(`audience.coverageKind.${row.key}`) }}
          </th>
          <td class="py-1 text-right tabular-nums">{{ n(row.value, 'int') }}</td>
        </tr>
      </tbody>
    </table>

    <table v-if="breakdown.length" class="w-full text-left text-xs">
      <caption class="text-muted-foreground pb-1 text-left font-medium">
        {{
          t('audience.breakdown')
        }}
      </caption>
      <thead>
        <tr class="text-muted-foreground">
          <th scope="col" class="py-1 pr-2 font-medium">{{ t('audience.language') }}</th>
          <th v-if="hasZone" scope="col" class="py-1 pr-2 font-medium">
            {{ t('audience.timezone') }}
          </th>
          <th scope="col" class="py-1 text-right font-medium">{{ t('audience.people') }}</th>
          <th scope="col" class="py-1 pl-2 text-right font-medium">
            {{ t('audience.coverageKind.both') }}
          </th>
          <th scope="col" class="py-1 pl-2 text-right font-medium">
            {{ t('audience.coverageKind.email_only') }}
          </th>
          <th scope="col" class="py-1 pl-2 text-right font-medium">
            {{ t('audience.coverageKind.sms_only') }}
          </th>
          <th scope="col" class="py-1 pl-2 text-right font-medium">
            {{ t('audience.coverageKind.unreachable') }}
          </th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="row in breakdown"
          :key="`${row.language}|${row.timezone ?? ''}`"
          class="border-t"
        >
          <th scope="row" class="py-1 pr-2 font-normal">{{ labels.language(row.language) }}</th>
          <td v-if="hasZone" class="py-1 pr-2 break-all">{{ row.timezone }}</td>
          <td class="py-1 text-right tabular-nums">{{ n(row.count, 'int') }}</td>
          <td class="py-1 pl-2 text-right tabular-nums">{{ n(row.coverage.both, 'int') }}</td>
          <td class="py-1 pl-2 text-right tabular-nums">{{ n(row.coverage.email_only, 'int') }}</td>
          <td class="py-1 pl-2 text-right tabular-nums">{{ n(row.coverage.sms_only, 'int') }}</td>
          <td class="py-1 pl-2 text-right tabular-nums">
            {{ n(row.coverage.unreachable, 'int') }}
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
