// User story: As an admin operator, I can open the dashboard and see the
// high-level server, device, and firmware overview backed by real seeded data.
package ui_test

import (
	"testing"
)

func adminDashboardStories() []Story {
	return []Story{{
		Name: "100-admin-dashboard",
		Run: func(_ testing.TB, page *Page) {
			page.GotoAdmin("/")
			page.ExpectText("Dashboard")
			page.ExpectText("Server Build")
			page.ExpectText("Peers This Page")
			page.ExpectText("Firmware Depots")
			page.ExpectText(SeedDepotName)
			page.ExpectText("Peers")
			page.ExpectText("Firmware")
		},
	}}
}
