// Command proposal is the proposal service (D-018, D-056).
package main

import "github.com/furknozg/coldbru/internal/platform/app"

func main() {
	app.Main(app.Options{Name: "proposal", Database: true, PlatformMigrations: true, NATS: true})
}
