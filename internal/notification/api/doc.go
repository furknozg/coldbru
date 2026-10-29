// Package api is Notification's live REST and SSE surface for the browser (D-013, D-036, D-080): the
// current delivery counts of a campaign and a Server-Sent Events stream that repeats them whenever
// they change. The handlers depend on the small Source interface, so tests drive them with a fake.
// Errors are RFC 9457 application/problem+json. There is no authentication in the demo (D-072);
// Notification is reachable only behind nginx. The validate:"required" tags on the DTOs are read by
// swag only: they mark a property as always present in the OpenAPI document and enforce nothing at
// run time.
package api
