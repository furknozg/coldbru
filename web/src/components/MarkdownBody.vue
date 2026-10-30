<script setup lang="ts">
import { computed } from 'vue'

import { renderMarkdown } from '@/lib/markdown'

/**
 * Agent text (a question, a summary) drawn as markdown (D-088). `source` has been through
 * `renderMarkdown`, which disables raw HTML and sanitises the output, before it reaches `v-html`.
 */
const props = defineProps<{ source: string }>()

const html = computed(() => renderMarkdown(props.source))
</script>

<template>
  <!-- eslint-disable-next-line vue/no-v-html -- the HTML comes from renderMarkdown, never raw -->
  <div class="markdown min-w-0 text-sm break-words" data-markdown v-html="html" />
</template>

<style scoped>
.markdown :deep(p:not(:last-child)),
.markdown :deep(ul:not(:last-child)),
.markdown :deep(ol:not(:last-child)),
.markdown :deep(blockquote:not(:last-child)),
.markdown :deep(pre:not(:last-child)) {
  margin-bottom: 0.5rem;
}
.markdown :deep(ul) {
  list-style: disc;
  padding-left: 1.25rem;
}
.markdown :deep(ol) {
  list-style: decimal;
  padding-left: 1.25rem;
}
.markdown :deep(h1),
.markdown :deep(h2),
.markdown :deep(h3),
.markdown :deep(h4),
.markdown :deep(h5),
.markdown :deep(h6) {
  font-weight: 600;
  margin-bottom: 0.25rem;
}
.markdown :deep(a) {
  color: var(--primary);
  text-decoration: underline;
  text-underline-offset: 2px;
}
.markdown :deep(code) {
  font-family: ui-monospace, monospace;
  font-size: 0.8em;
  background: var(--muted);
  border-radius: 0.25rem;
  padding: 0 0.25rem;
}
.markdown :deep(pre) {
  overflow-x: auto;
  background: var(--muted);
  border-radius: 0.5rem;
  padding: 0.5rem;
}
.markdown :deep(blockquote) {
  border-left: 3px solid var(--border);
  padding-left: 0.75rem;
  color: var(--muted-foreground);
}
.markdown :deep(table) {
  border-collapse: collapse;
}
.markdown :deep(th),
.markdown :deep(td) {
  border: 1px solid var(--border);
  padding: 0.125rem 0.5rem;
}
</style>
