// User story: As a Play UI user, I can switch the video-call shell between
// supported visual themes.
package ui_test

import (
	"testing"
)

func playActionsStories() []Story {
	return []Story{{
		Name: "201-play-actions",
		Run: func(_ testing.TB, page *Page) {
			page.GotoPlay("/")
			page.ClickRole("button", "Dark")
			page.ExpectText("Start Video Call")
			page.ClickRole("button", "Light")
			page.ExpectText("Start Video Call")
		},
	}}
}
