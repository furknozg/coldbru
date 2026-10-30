<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

/** Asks before something that discards work; the body slot can hold one more field (a reason). */
const {
  title,
  description,
  confirmLabel,
  busy = false,
  destructive = false,
  canConfirm = true,
} = defineProps<{
  title: string
  description: string
  confirmLabel: string
  busy?: boolean
  /** The confirm button is drawn as a warning when the action cannot be taken back. */
  destructive?: boolean
  /** False while the body slot holds something that is not valid yet (a reason that is too long). */
  canConfirm?: boolean
}>()

const emit = defineEmits<{ confirm: [] }>()
const open = defineModel<boolean>('open', { required: true })

const { t } = useI18n()
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>{{ title }}</DialogTitle>
        <DialogDescription>{{ description }}</DialogDescription>
      </DialogHeader>
      <slot />
      <DialogFooter>
        <Button variant="outline" type="button" :disabled="busy" @click="open = false">
          {{ t('common.cancel') }}
        </Button>
        <Button
          type="button"
          :variant="destructive ? 'destructive' : 'default'"
          :disabled="busy || !canConfirm"
          @click="emit('confirm')"
        >
          {{ busy ? t('common.sending') : confirmLabel }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
