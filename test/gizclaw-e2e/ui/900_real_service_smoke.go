// User story: As a maintainer, I can run one smoke path that proves Admin and
// Play both serve against the same seeded test service.
package ui_test

import (
	"testing"
)

func realServiceSmokeStories() []Story {
	return []Story{{
		Name: "900-real-service-smoke",
		Run: func(_ testing.TB, page *Page) {
			page.GotoAdmin("/")
			page.ExpectText("Dashboard")
			page.ExpectText(SeedDepotName)

			page.GotoPlay("/")
			page.ClickRole("button", "Start Video Call")
			page.ExpectText("rpc.response")
			page.ExpectText("build_commit")
		},
	}}
}
