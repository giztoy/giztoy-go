// User story: As an admin operator, I can inspect a seeded peer across its
// info, config, runtime, OTA, and raw views.
package ui_test

import (
	"net/url"
	"testing"
)

func adminPeerDetailStories() []Story {
	return []Story{{
		Name: "111-admin-peer-detail",
		Run: func(_ testing.TB, page *Page) {
			page.GotoAdmin("/peers/" + url.PathEscape(page.Seed.DevicePublicKey))
			page.ExpectText("Seeded UI Device")
			page.ExpectText(page.Seed.DevicePublicKey)
			page.ExpectText("Peer Actions")
			page.ExpectText("Firmware Policy")
			page.ExpectText("ui-device-sn")

			page.ClickRole("tab", "Config")
			page.ExpectText("Configuration")
			page.ExpectText("ui-cert")

			page.ClickRole("tab", "Runtime")
			page.ExpectText("Last Address")
			page.ExpectText("Online")

			page.ClickRole("tab", "OTA")
			page.ExpectText("firmware_semver")

			page.ClickRole("tab", "Raw")
			page.ExpectText("Seeded UI Device")
		},
	}}
}
