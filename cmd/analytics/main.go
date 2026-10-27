// Command analytics is the analytics service (D-018, D-056).
package main

import "github.com/furknozg/coldbru/internal/platform/app"

func main() {
	app.Main(app.Options{Name: "analytics", Database: true, PlatformMigrations: true, NATS: true})
}
