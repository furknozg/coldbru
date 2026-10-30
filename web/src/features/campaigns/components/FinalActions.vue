<script setup lang="ts">
import { RefreshCwIcon, SendIcon } from '@lucide/vue'
import { computed, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Button } from '@/components/ui/button'

import { useActions } from '../composables/campaignActions'
import type { CampaignStatus, StepView } from '../types'

/**
 * The accordion at the end of the chat (D-091): what can be done to a drafting campaign as a whole.
 * Retry a confirmation that failed, restart a start that never produced a step. It is not drawn
 * when neither is possible; cancelling is in the chat bar and reopening a step is in that step's
 * accordion.
 */
const props = defineProps<{ status: CampaignStatus; steps: readonly StepView[] }>()

const { t } = useI18n()
const actions = useActions()

const ITEM = 'actions'
const drafting = computed(() => props.status === 'drafting')
const canRestart = computed(() => drafting.value && props.steps.length === 0)
const canRetryConfirm = computed(
  () => drafting.value && props.steps.find((s) => s.kind === 'schedule')?.status === 'approved',
)
const available = computed(() => canRestart.value || canRetryConfirm.value)

// A start or a confirmation that did not go through asks for attention, so it opens unasked.
const chosen = shallowRef<boolean | null>(null)
const expanded = computed(() => chosen.value ?? true)
watch(
  () => [canRestart.value, canRetryConfirm.value],
  () => {
    chosen.value = null
  },
)
</script>

<template>
  <li v-if="available" class="min-w-0" data-kind="campaignActions">
    <Accordion
      type="single"
      collapsible
      :model-value="expanded ? ITEM : ''"
      @update:model-value="chosen = $event === ITEM"
    >
      <AccordionItem :value="ITEM" class="border-b-0">
        <AccordionTrigger class="items-center justify-start gap-2 py-1 text-xs">
          {{ t('campaign.actionsTitle') }}
        </AccordionTrigger>
        <AccordionContent>
          <div class="flex flex-col gap-3 text-sm">
            <div v-if="canRestart" class="flex flex-col items-start gap-2">
              <p class="text-muted-foreground">{{ t('campaign.noSteps') }}</p>
              <Button
                size="sm"
                variant="outline"
                :disabled="actions.busy.value"
                @click="actions.regenerateCampaign()"
              >
                <RefreshCwIcon />
                {{
                  actions.pending.value === 'regenerate-campaign'
                    ? t('common.sending')
                    : t('campaign.restart')
                }}
              </Button>
            </div>

            <div v-if="canRetryConfirm" class="flex flex-col items-start gap-2">
              <p class="text-muted-foreground">{{ t('campaign.confirmPending') }}</p>
              <Button size="sm" :disabled="actions.busy.value" @click="actions.confirm()">
                <SendIcon />
                {{
                  actions.pending.value === 'confirm'
                    ? t('common.sending')
                    : t('campaign.retryConfirm')
                }}
              </Button>
            </div>
          </div>
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  </li>
</template>
