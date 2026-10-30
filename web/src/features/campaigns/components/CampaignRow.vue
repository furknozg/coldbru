<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { CampaignInfo } from '../types'

/** One campaign in the sidebar (D-091): the title (or the brief) and the short date on one line. */
defineProps<{ campaign: CampaignInfo; selected: boolean }>()

const emit = defineEmits<{ navigate: [] }>()

const { d } = useI18n()
</script>

<template>
  <li>
    <!-- no-underline! (important): the accordion content underlines every link inside it. -->
    <RouterLink
      :to="{ name: 'campaign', params: { id: campaign.id } }"
      class="hover:bg-accent focus-visible:ring-ring/50 no-underline! flex items-baseline justify-between gap-2 rounded-md px-2 py-1 text-sm outline-none focus-visible:ring-3"
      :class="selected ? 'bg-accent' : ''"
      :aria-current="selected ? 'page' : undefined"
      @click="emit('navigate')"
    >
      <span
        class="line-clamp-2 min-w-0 flex-1 break-words"
        :class="campaign.title ? 'font-medium' : ''"
        :title="campaign.title ? campaign.brief : undefined"
      >
        {{ campaign.title || campaign.brief }}
      </span>
      <time class="text-muted-foreground shrink-0 text-xs" :datetime="campaign.created_at">
        {{ d(campaign.created_at, 'date') }}
      </time>
    </RouterLink>
  </li>
</template>
