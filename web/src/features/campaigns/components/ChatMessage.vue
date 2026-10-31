<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import MarkdownBody from '@/components/MarkdownBody.vue'
import { Button } from '@/components/ui/button'

import { useActions } from '../composables/campaignActions'
import type { TimelineItem } from '../timeline'

/**
 * A message of the chat (D-091): a role label with the time and the text under it, no box. What
 * the marketer wrote is right-aligned as a whole, label row and text (D-101); the agent's stays
 * on the left. The
 * marketer's text is drawn literally; the agent's (`item.markdown`) as sanitised markdown (D-088).
 * A question that still waits for an answer shows its suggested answers as buttons (D-090). The
 * line fades in once, when it is added (D-112).
 */
const props = defineProps<{
  item: TimelineItem
  /** The item is a question without an answer, in a step that still takes one. */
  pending?: boolean
}>()

const { t, d } = useI18n()
const actions = useActions()

async function choose(option: string) {
  if (props.item.question) await actions.answer(props.item.question.id, option)
}
</script>

<template>
  <li
    class="flex min-w-0 flex-col gap-0.5 text-sm motion-safe:animate-in motion-safe:fade-in motion-safe:duration-300"
    :class="item.author === 'user' ? 'items-end text-right' : ''"
    :data-kind="item.kind"
    :data-author="item.author"
    :data-pending="pending || undefined"
  >
    <p class="flex items-baseline gap-2" :class="item.author === 'user' ? 'justify-end' : ''">
      <span class="font-medium" :class="item.author === 'user' ? 'text-espresso' : 'text-caramel'">
        {{ item.title }}
      </span>
      <time class="text-muted-foreground shrink-0 text-xs" :datetime="item.at">
        {{ d(item.at, 'time') }}
      </time>
    </p>
    <MarkdownBody v-if="item.body && item.markdown" :source="item.body" />
    <p
      v-else-if="item.body"
      class="break-words whitespace-pre-wrap"
      :class="item.author === 'user' ? 'max-w-[85%] text-right' : ''"
    >
      {{ item.body }}
    </p>
    <div
      v-if="pending && item.question && item.question.options.length"
      class="mt-1 flex flex-wrap gap-2"
      role="group"
      :aria-label="t('question.options')"
    >
      <Button
        v-for="option in item.question.options"
        :key="option"
        size="sm"
        variant="outline"
        :disabled="actions.busy.value"
        @click="choose(option)"
      >
        {{ actions.pending.value === `answer:${item.question.id}` ? t('common.sending') : option }}
      </Button>
    </div>
    <p v-if="pending" class="text-muted-foreground text-xs">{{ t('question.hint') }}</p>
  </li>
</template>
