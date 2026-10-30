<script setup lang="ts">
import { CheckIcon, PencilIcon, XIcon } from '@lucide/vue'
import { computed, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

import { useActions } from '../composables/campaignActions'
import { useLabels } from '../labels'
import type { Problem } from '../problem'
import type { StepKind, VariantView } from '../types'
import AudienceBody from './AudienceBody.vue'
import AudienceEditForm from './AudienceEditForm.vue'
import ContentBody from './ContentBody.vue'
import EmailEditForm from './EmailEditForm.vue'
import PlanBody from './PlanBody.vue'
import PlanEditForm from './PlanEditForm.vue'
import RejectDialog from './RejectDialog.vue'
import ScheduleBody from './ScheduleBody.vue'

/**
 * One proposal of a step, and the only card of the chat (D-091): what the model (or the marketer's
 * edit) produced, its state and, while it is a draft in the open step, the actions: approve, reject
 * with a reason, and edit where the step allows it (audience, channel plan and email text; SMS and
 * schedule editing arrive with T-036). Nothing inside it is a box of its own. A rejected variant is
 * not a card; its step lists it in the accordion.
 */
const props = defineProps<{
  variant: VariantView
  kind: StepKind
  /** A draft of the open step in a campaign that is still drafting. */
  actionable: boolean
}>()

const { t } = useI18n()
const labels = useLabels()
const actions = useActions()

const editing = shallowRef(false)
const rejecting = shallowRef(false)
const editProblem = shallowRef<Problem | null>(null)
const rejectProblem = shallowRef<Problem | null>(null)

const isEmail = computed(() => props.kind === 'content' && props.variant.channel === 'email')
const editable = computed(
  () => props.kind === 'audience' || props.kind === 'channel_plan' || isEmail.value,
)

const title = computed(() => {
  const slot = labels.slot({
    language: props.variant.language,
    channel: props.variant.channel,
    timezone: props.variant.timezone,
  })
  return slot || labels.stepKind(props.kind)
})

const statusTone: Record<VariantView['status'], string> = {
  draft: 'border-caramel/50 text-espresso',
  approved: 'bg-sage/30 text-espresso border-transparent',
  rejected: 'bg-muted text-muted-foreground',
  superseded: 'bg-muted text-muted-foreground',
}

const isBusy = (key: string) => actions.pending.value === key

async function approve() {
  await actions.approve(props.variant.id)
}

async function reject(reason: string) {
  const result = await actions.reject(props.variant.id, reason)
  if (result.ok) {
    rejecting.value = false
    rejectProblem.value = null
  } else {
    rejectProblem.value = result.problem.status === 422 ? result.problem : null
  }
}

async function save(content: Record<string, unknown>) {
  const result = await actions.edit(props.variant.id, content)
  if (result.ok) {
    editing.value = false
    editProblem.value = null
  } else {
    editProblem.value = result.problem.status === 422 ? result.problem : null
  }
}

function openEdit() {
  editProblem.value = null
  editing.value = true
}

function openReject() {
  rejectProblem.value = null
  rejecting.value = true
}
</script>

<template>
  <article
    class="flex flex-col gap-3 rounded-xl border p-3"
    data-slot="card"
    :aria-label="title"
    :data-variant-status="variant.status"
  >
    <header class="flex items-center justify-between gap-2">
      <h4 class="text-sm font-semibold break-words">{{ title }}</h4>
      <Badge variant="outline" :class="statusTone[variant.status]">
        {{ t(`variant.status.${variant.status}`) }}
      </Badge>
    </header>

    <AudienceEditForm
      v-if="editing && kind === 'audience'"
      :variant="variant"
      :busy="isBusy(`edit:${variant.id}`)"
      :problem="editProblem"
      @save="save"
      @cancel="editing = false"
    />
    <PlanEditForm
      v-else-if="editing && kind === 'channel_plan'"
      :variant="variant"
      :busy="isBusy(`edit:${variant.id}`)"
      :problem="editProblem"
      @save="save"
      @cancel="editing = false"
    />
    <EmailEditForm
      v-else-if="editing && isEmail"
      :variant="variant"
      :busy="isBusy(`edit:${variant.id}`)"
      :problem="editProblem"
      @save="save"
      @cancel="editing = false"
    />
    <template v-else>
      <AudienceBody v-if="kind === 'audience'" :variant="variant" />
      <PlanBody v-else-if="kind === 'channel_plan'" :variant="variant" />
      <ContentBody v-else-if="kind === 'content'" :variant="variant" />
      <ScheduleBody v-else :variant="variant" />
    </template>

    <footer v-if="actionable && !editing" class="flex flex-wrap gap-2">
      <Button size="sm" :disabled="actions.busy.value" @click="approve">
        <CheckIcon />{{
          isBusy(`approve:${variant.id}`) ? t('common.sending') : t('variant.approve')
        }}
      </Button>
      <Button size="sm" variant="outline" :disabled="actions.busy.value" @click="openReject">
        <XIcon />{{ t('variant.reject') }}
      </Button>
      <Button
        v-if="editable"
        size="sm"
        variant="outline"
        :disabled="actions.busy.value"
        @click="openEdit"
      >
        <PencilIcon />{{ t('variant.edit') }}
      </Button>
    </footer>

    <RejectDialog
      v-model:open="rejecting"
      :target="title"
      :busy="isBusy(`reject:${variant.id}`)"
      :problem="rejectProblem"
      @reject="reject"
    />
  </article>
</template>
