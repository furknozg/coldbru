// Package api is the REST surface of the redirect service (D-106, D-107): POST /links, which
// Notification calls while it writes the delivery rows of a recipient batch, and GET /r/{code},
// the short link that SMS and email recipients click. Errors of POST /links are RFC 9457
// application/problem+json. POST /links is reachable only inside the compose network (nginx routes
// GET /r/ alone) and has no authentication, like the other service-to-service calls of the demo.
// The validate:"required" tags on the DTOs are read by swag only: they mark a property as always
// present in the OpenAPI document and enforce nothing at run time.
package api
