<script setup lang="ts">
import { XIcon } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'
import { useToasts } from '@/features/campaigns/composables/useToasts'

const { t } = useI18n()
const { toasts, dismiss } = useToasts()
</script>

<template>
  <div
    class="pointer-events-none fixed inset-x-4 bottom-4 z-[60] flex flex-col items-end gap-2"
    aria-live="polite"
  >
    <div
      v-for="toast in toasts"
      :key="toast.id"
      :role="toast.tone === 'error' ? 'alert' : 'status'"
      class="bg-popover text-popover-foreground ring-foreground/10 pointer-events-auto flex w-full max-w-sm items-start gap-2 rounded-xl border-l-4 p-3 text-sm shadow-lg ring-1"
      :class="toast.tone === 'error' ? 'border-l-destructive' : 'border-l-primary'"
    >
      <p class="min-w-0 flex-1 break-words">{{ toast.message }}</p>
      <Button
        variant="ghost"
        size="icon-xs"
        :aria-label="t('common.dismiss')"
        @click="dismiss(toast.id)"
      >
        <XIcon />
      </Button>
    </div>
  </div>
</template>
