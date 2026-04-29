// User story: As a maintainer, I can run one smoke path that proves Admin still
// talks to the real service while Play renders the dial surface.
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
			page.ExpectText("Start Video Call")
		},
	}}
}
