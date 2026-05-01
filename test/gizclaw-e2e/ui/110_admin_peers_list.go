// User story: As an admin operator, I can list registered peers and filter
// the current inventory page using real peer registrations.
package ui_test

import (
	"testing"
)

func adminPeersListStories() []Story {
	return []Story{{
		Name: "110-admin-peers-list",
		Run: func(_ testing.TB, page *Page) {
			page.GotoAdmin("/peers")
			page.ExpectText("Peers")
			page.ExpectText("Peer Inventory")
			page.ExpectText(page.Seed.DevicePublicKey)
			page.ExpectText("gear")
			page.ExpectText("Active")

			page.Fill(`input[placeholder="Filter current page by key, role, or status"]`, "missing")
			page.ExpectText("No matching peers")
			page.Fill(`input[placeholder="Filter current page by key, role, or status"]`, page.Seed.DevicePublicKey[:12])
			page.ExpectText(page.Seed.DevicePublicKey)
		},
	}}
}
