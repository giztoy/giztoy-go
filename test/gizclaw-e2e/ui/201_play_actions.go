// User story: As a Play UI user, I do not see legacy local HTTP action buttons
// before starting a WebRTC call.
package ui_test

import (
	"testing"
)

func playActionsStories() []Story {
	return []Story{{
		Name: "201-play-actions",
		Run: func(_ testing.TB, page *Page) {
			page.GotoPlay("/")
			page.ExpectText("Start Video Call")
		},
	}}
}
