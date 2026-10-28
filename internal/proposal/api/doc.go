// Package api is Proposal's REST and SSE surface for the browser (D-013, D-016, D-072): resource
// and action endpoints over the flow service, RFC 9457 problem+json errors, and the live event
// stream of one campaign. The handlers depend on the small Flow interface, so tests drive them with
// a fake. The validate:"required" tags on the DTOs (here and in package flow) are read by swag only:
// they mark a property as always present in the OpenAPI document and enforce nothing at run time.
// There is no authentication in the demo (D-072); Proposal is reachable only behind nginx.
package api
