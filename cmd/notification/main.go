// Command notification is the notification service (D-018, D-056).
package main

import "github.com/furknozg/coldbru/internal/platform/app"

func main() {
	app.Main(app.Options{Name: "notification", Database: true, PlatformMigrations: true, NATS: true})
}
