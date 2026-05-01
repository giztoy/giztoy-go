// User story: As an admin operator, I can perform peer actions against real
// seeded registrations, including approve, refresh, channel save, block, and delete.
package ui_test

import (
	"net/url"
	"testing"
)

func adminPeerActionsStories() []Story {
	return []Story{
		{
			Name: "112-admin-peer-actions",
			Run: func(_ testing.TB, page *Page) {
				page.GotoAdmin("/peers/" + url.PathEscape(page.Seed.ActionDevicePublicKey))
				page.ClickRole("button", "Approve")
				page.ExpectText("Peer approved as gear.")
				page.ClickRole("button", "Refresh")
				page.ExpectText("Peer refreshed.")
				page.ClickRole("button", "Save Channel")
				page.ExpectText("Desired channel updated to stable.")
				page.ClickRole("button", "Block")
				page.ExpectText("Peer blocked.")
			},
		},
		{
			Name: "112-admin-peer-delete",
			Run: func(_ testing.TB, page *Page) {
				page.GotoAdmin("/peers/" + url.PathEscape(page.Seed.DeleteDevicePublicKey))
				page.ClickRole("button", "Delete")
				page.ExpectURLSuffix("/peers")
				page.ExpectText("Peers")
			},
		},
	}
}
