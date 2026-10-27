// Command mockprovider is the mock SMS/email provider (D-018, D-056).
package main

import "github.com/furknozg/coldbru/internal/platform/app"

func main() {
	app.Main(app.Options{Name: "mockprovider"})
}
