import { describe, expectTypeOf, it } from 'vitest'

import type { NotificationEvents, ProposalEvents } from '../events'

// The event maps are derived from the generated schemas, so a renamed or removed event shows up
// here as a type error in `pnpm type-check` (and the freshness test fails until the schemas are
// regenerated). Update the lists together with the handlers that consume the events.
describe('event catalogues', () => {
  it('lists the proposal events', () => {
    expectTypeOf<keyof ProposalEvents>().toEqualTypeOf<
      | 'campaign_cancelled'
      | 'campaign_confirmed'
      | 'campaign_status'
      | 'content_delta'
      | 'content_reset'
      | 'context_added'
      | 'generation_failed'
      | 'generation_started'
      | 'ping'
      | 'screen_result'
      | 'snapshot'
      | 'step_approved'
      | 'step_opened'
      | 'step_superseded'
      | 'variant_approved'
      | 'variant_created'
      | 'variant_rejected'
      | 'variant_superseded'
    >()
  })

  it('lists the notification events and their payloads', () => {
    expectTypeOf<keyof NotificationEvents>().toEqualTypeOf<'done' | 'ping' | 'snapshot'>()
    expectTypeOf<NotificationEvents['done']['status']>().toEqualTypeOf<'completed' | 'cancelled'>()
    expectTypeOf<NotificationEvents['ping']>().toEqualTypeOf<{ at: string }>()
  })
})
