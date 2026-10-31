import type {
  CampaignReport,
  LlmSeries,
  Overview,
  OverviewSeries,
  PromptVersionUsage,
} from '@/api/analytics/service'

/** Analytics answers for the dashboard tests: small, with round numbers that are easy to read back. */
export const overviewOf = (over: Partial<Overview> = {}): Overview => ({
  campaigns_completed: 12,
  deliveries: [
    { channel: 'email', status: 'sent', count: 800 },
    { channel: 'email', status: 'failed', count: 30 },
    { channel: 'sms', status: 'sent', count: 400 },
    { channel: 'sms', status: 'failed', count: 4 },
    { channel: 'sms', status: 'skipped_no_consent', count: 66 },
  ],
  delivery_cost_usd: '96.000000',
  from: null as unknown as string,
  to: null as unknown as string,
  sms_segments: 2400,
  llm: {
    calls: 56,
    cost_usd: '0.01234500',
    tokens: { input: 100000, output: 20000, cache_read: 300000, cache_creation: 50000 },
    by_purpose: [
      {
        purpose: 'screen',
        calls: 10,
        cost_usd: '0.00100000',
        input_tokens: 1000,
        output_tokens: 100,
      },
      {
        purpose: 'content',
        calls: 30,
        cost_usd: '0.01000000',
        input_tokens: 90000,
        output_tokens: 19000,
      },
    ],
    by_model: [
      {
        model: 'claude-haiku-5-5',
        calls: 10,
        cost_usd: '0.00100000',
        input_tokens: 1000,
        output_tokens: 100,
      },
      {
        model: 'claude-sonnet-5-5',
        calls: 46,
        cost_usd: '0.01134500',
        input_tokens: 99000,
        output_tokens: 19900,
      },
    ],
  },
  clicks: {
    sent: 1200,
    unique_clicks: 150,
    total_clicks: 210,
    ctr: 0.125,
    by_channel: [
      { channel: 'email', sent: 800, unique_clicks: 120, total_clicks: 170, ctr: 0.15 },
      { channel: 'sms', sent: 400, unique_clicks: 30, total_clicks: 40, ctr: 0.075 },
    ],
  },
  ...over,
})

const DAYS = ['2026-10-28T00:00:00Z', '2026-10-29T00:00:00Z', '2026-10-30T00:00:00Z']

export const overviewSeriesOf = (): OverviewSeries => ({
  bucket: 'day',
  buckets: DAYS,
  from: DAYS[0]!,
  to: '2026-10-31T00:00:00Z',
  points: [
    { start: DAYS[1]!, channel: 'email', status: 'sent', count: 10 },
    { start: DAYS[1]!, channel: 'sms', status: 'sent', count: 6 },
    { start: DAYS[2]!, channel: 'email', status: 'sent', count: 5 },
    { start: DAYS[2]!, channel: 'email', status: 'failed', count: 9 },
  ],
  clicks: [
    { start: DAYS[1]!, channel: 'email', unique_clicks: 4, total_clicks: 6 },
    { start: DAYS[2]!, channel: 'email', unique_clicks: 3, total_clicks: 3 },
    { start: DAYS[2]!, channel: 'sms', unique_clicks: 2, total_clicks: 5 },
  ],
})

export const llmSeriesOf = (): LlmSeries => ({
  bucket: 'day',
  buckets: DAYS,
  from: DAYS[0]!,
  to: '2026-10-31T00:00:00Z',
  points: [
    {
      start: DAYS[0]!,
      purpose: 'screen',
      model: 'claude-haiku-5-5',
      calls: 4,
      cost_usd: '0.00100000',
      input_tokens: 400,
      output_tokens: 40,
    },
    {
      start: DAYS[2]!,
      purpose: 'content',
      model: 'claude-sonnet-5-5',
      calls: 7,
      cost_usd: '0.00900000',
      input_tokens: 7000,
      output_tokens: 700,
    },
  ],
})

export const promptVersionsOf = (): PromptVersionUsage[] => [
  {
    prompt_name: 'content',
    prompt_version: '1.2.0',
    calls: 20,
    accepted: 17,
    validation_failed: 1,
    refusal: 1,
    api_error: 1,
    repair_rate: 0.05,
    avg_latency_ms: 800,
    cost_usd: '0.01230000',
  },
  {
    prompt_name: 'screen',
    prompt_version: '1.0.0',
    calls: 10,
    accepted: 10,
    validation_failed: 0,
    refusal: 0,
    api_error: 0,
    repair_rate: 0,
    avg_latency_ms: 300,
    cost_usd: '0.00100000',
  },
]

export const reportOf = (over: Partial<CampaignReport> = {}): CampaignReport => ({
  campaign_id: 'c1',
  completed_at: '2026-10-30T10:00:00Z',
  deliveries: [
    { channel: 'email', status: 'sent', count: 90 },
    { channel: 'email', status: 'failed', count: 10 },
    { channel: 'sms', status: 'sent', count: 50 },
  ],
  delivery_cost_usd: '12.500000',
  sms_segments: 75,
  llm: {
    calls: 6,
    cost_usd: '0.00420000',
    tokens: { input: 5000, output: 1000, cache_read: 0, cache_creation: 0 },
    by_purpose: [
      { purpose: 'screen', calls: 1, cost_usd: '0.00020000', input_tokens: 100, output_tokens: 10 },
      {
        purpose: 'content',
        calls: 5,
        cost_usd: '0.00400000',
        input_tokens: 4900,
        output_tokens: 990,
      },
    ],
  },
  clicks: {
    sent: 140,
    unique_clicks: 20,
    total_clicks: 31,
    ctr: 0.1429,
    by_channel: [
      { channel: 'email', sent: 90, unique_clicks: 15, total_clicks: 24, ctr: 0.1667 },
      { channel: 'sms', sent: 50, unique_clicks: 5, total_clicks: 7, ctr: 0.1 },
    ],
    by_locale: [
      {
        channel: 'email',
        language: 'tr',
        timezone: 'Europe/Istanbul',
        sent: 60,
        unique_clicks: 12,
        total_clicks: 20,
        ctr: 0.2,
      },
      {
        channel: 'email',
        language: 'en',
        timezone: 'Europe/London',
        sent: 30,
        unique_clicks: 3,
        total_clicks: 4,
        ctr: 0.1,
      },
      {
        channel: 'sms',
        language: 'tr',
        timezone: 'Europe/Istanbul',
        sent: 50,
        unique_clicks: 5,
        total_clicks: 7,
        ctr: 0.1,
      },
    ],
  },
  ...over,
})
