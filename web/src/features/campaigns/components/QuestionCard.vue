<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import MarkdownBody from '@/components/MarkdownBody.vue'
import { Button } from '@/components/ui/button'

import { useActions } from '../composables/campaignActions'
import type { TimelineItem } from '../timeline'

/**
 * A question of the agent (D-088, D-090): the question as markdown and, while it waits for an
 * answer, its suggested answers as buttons. Free text goes through the composer under the chat.
 */
const props = defineProps<{
  item: TimelineItem & { question: NonNullable<TimelineItem['question']> }
  /** The question has no answer yet and the step still takes one. */
  pending: boolean
}>()

const { t, d } = useI18n()
const actions = useActions()

async function choose(option: string) {
  await actions.answer(props.item.question.id, option)
}
</script>

<template>
  <li class="flex justify-start" :data-kind="item.kind" :data-pending="pending || undefined">
    <div
      class="border-l-caramel bg-muted/60 max-w-[85%] min-w-0 rounded-xl border border-l-4 px-3 py-2 text-sm"
    >
      <span class="flex items-baseline justify-between gap-3">
        <span class="font-medium">{{ item.title }}</span>
        <time class="text-muted-foreground shrink-0 text-xs" :datetime="item.at">
          {{ d(item.at, 'time') }}
        </time>
      </span>
      <MarkdownBody v-if="item.body" class="mt-1" :source="item.body" />
      <div
        v-if="pending && item.question.options.length"
        class="mt-2 flex flex-wrap gap-2"
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
          {{
            actions.pending.value === `answer:${item.question.id}` ? t('common.sending') : option
          }}
        </Button>
      </div>
      <p v-if="pending" class="text-muted-foreground mt-2 text-xs">{{ t('question.hint') }}</p>
    </div>
  </li>
</template>
