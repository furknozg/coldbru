// Package api is Analytics' report REST surface (D-036, D-083): completed campaign summaries, the
// delivery counts, SMS segments and costs of one campaign, and the LLM usage per prompt version.
// The handlers depend on the small Source interface, so tests drive them with a fake. Errors are
// RFC 9457 application/problem+json. There is no authentication in the demo (D-072). Money is a
// decimal string in US dollars, never a JSON number. The validate:"required" tags on the DTOs are
// read by swag only: they mark a property as always present in the OpenAPI document and enforce
// nothing at run time.
package api
