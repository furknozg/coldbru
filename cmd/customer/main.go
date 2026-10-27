// Command customer is the customer service: sync GraphQL over customer_db, no NATS (D-011, D-043, D-056).
package main

import "github.com/furknozg/coldbru/internal/platform/app"

func main() {
	app.Main(app.Options{Name: "customer", Database: true})
}
