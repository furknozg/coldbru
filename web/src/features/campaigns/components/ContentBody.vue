<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { emailContent, smsContent } from '../content'
import type { VariantView } from '../types'

/**
 * A text proposal, read only here (editing is T-036). Every field is shown as plain text: the model
 * wrote it and the page never treats it as markup.
 */
const props = defineProps<{ variant: VariantView }>()

const { t, n } = useI18n()

const isSms = computed(() => props.variant.channel === 'sms')
const sms = computed(() => smsContent(props.variant))
const email = computed(() => emailContent(props.variant))
// The names of the two SMS alphabets are technical terms, not words to translate.
const ENCODINGS: Record<string, string> = { gsm7: 'GSM-7', ucs2: 'UCS-2' }
const encoding = computed(() => ENCODINGS[sms.value.encoding] ?? sms.value.encoding)
const cost = computed(() => (isSms.value ? sms.value.cost : email.value.cost))
</script>

<template>
  <div class="flex flex-col gap-2 text-sm">
    <template v-if="isSms">
      <p class="bg-muted rounded-lg p-2 break-words whitespace-pre-wrap">{{ sms.text }}</p>
      <dl class="text-muted-foreground grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
        <dt>{{ t('content.encoding') }}</dt>
        <dd class="text-foreground">{{ encoding }}</dd>
        <dt>{{ t('content.segments') }}</dt>
        <dd class="text-foreground tabular-nums">{{ n(sms.segments, 'int') }}</dd>
        <dt>{{ t('content.length') }}</dt>
        <dd class="text-foreground tabular-nums">{{ n(sms.renderedLength, 'int') }}</dd>
      </dl>
    </template>
    <template v-else>
      <dl class="flex flex-col gap-2">
        <div>
          <dt class="text-muted-foreground text-xs">{{ t('content.subject') }}</dt>
          <dd class="font-medium break-words whitespace-pre-wrap">{{ email.subject }}</dd>
        </div>
        <div>
          <dt class="text-muted-foreground text-xs">{{ t('content.preheader') }}</dt>
          <dd class="break-words whitespace-pre-wrap">{{ email.preheader }}</dd>
        </div>
        <div>
          <dt class="text-muted-foreground text-xs">{{ t('content.headline') }}</dt>
          <dd class="break-words whitespace-pre-wrap">{{ email.headline }}</dd>
        </div>
        <div>
          <dt class="text-muted-foreground text-xs">{{ t('content.paragraphs') }}</dt>
          <dd class="flex flex-col gap-1">
            <span
              v-for="(paragraph, index) in email.paragraphs"
              :key="index"
              class="bg-muted rounded-lg p-2 break-words whitespace-pre-wrap"
              >{{ paragraph }}</span
            >
          </dd>
        </div>
        <div>
          <dt class="text-muted-foreground text-xs">{{ t('content.cta') }}</dt>
          <dd class="break-words whitespace-pre-wrap">{{ email.ctaLabel }}</dd>
        </div>
      </dl>
    </template>
    <p v-if="cost" class="text-muted-foreground text-xs">
      {{ t('content.estimatedCost') }}:
      <span class="text-foreground tabular-nums">{{
        n(Number(cost.amount), cost.currency === 'USD' ? 'usd' : 'gbp')
      }}</span>
    </p>
  </div>
</template>
