<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { emailContent, smsContent } from '../content'
import type { VariantView } from '../types'
import EmailBody from './EmailBody.vue'

/**
 * A text proposal, read only here. An SMS is shown as plain text with its measurements; an email as
 * the server renders it (D-090). The model wrote every field and the page never treats it as markup.
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
      <p class="border-caramel/40 border-l-2 pl-2 break-words whitespace-pre-wrap">
        {{ sms.text }}
      </p>
      <dl class="text-muted-foreground grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
        <dt>{{ t('content.encoding') }}</dt>
        <dd class="text-foreground">{{ encoding }}</dd>
        <dt>{{ t('content.segments') }}</dt>
        <dd class="text-foreground tabular-nums">{{ n(sms.segments, 'int') }}</dd>
        <dt>{{ t('content.length') }}</dt>
        <dd class="text-foreground tabular-nums">{{ n(sms.renderedLength, 'int') }}</dd>
      </dl>
    </template>
    <EmailBody v-else :variant="variant" />
    <p v-if="cost" class="text-muted-foreground text-xs">
      {{ t('content.estimatedCost') }}:
      <span class="text-foreground tabular-nums">{{
        n(Number(cost.amount), cost.currency === 'USD' ? 'usd' : 'gbp')
      }}</span>
    </p>
  </div>
</template>
