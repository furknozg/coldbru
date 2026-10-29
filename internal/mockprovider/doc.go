// Package mockprovider is the mock SMS and email provider (D-018, D-081). It implements the contract
// Notification's client expects (internal/notification/provider, D-079):
//
//	POST /v1/sms    {"to", "text"}
//	POST /v1/email  {"to", "subject", "html", "text"}
//	header Idempotency-Key: <delivery id>   (required)
//	202 {"message_id"}   accepted
//	429 + Retry-After    rate limited, or an injected 429
//	5xx                  injected failure, or a failed forward
//	4xx                  invalid request, permanent
//
// A request runs through these steps, in order: idempotency lookup, validation, a per-channel token
// bucket, fault injection, latency, forwarding to the inbox (email to Mailpit over SMTP, SMS to
// MockServer over HTTP), and storing the answer under the Idempotency-Key for 24 hours. A repeated key
// gets the stored answer and is forwarded no second time. There is no delivery receipt: 202 means sent.
package mockprovider
