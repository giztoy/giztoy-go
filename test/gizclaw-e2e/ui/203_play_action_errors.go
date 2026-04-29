// User story: As a Play UI user, the error fixture still renders the WebRTC
// dial shell without relying on legacy proxied HTTP action buttons.
package ui_test

import (
	"testing"
)

func playActionErrorsStories() []Story {
	return []Story{{
		Name: "203-play-action-errors",
		Run: func(_ testing.TB, page *Page) {
			page.GotoErrorPlay("/")
			page.ExpectText("Start Video Call")
		},
	}}
}
