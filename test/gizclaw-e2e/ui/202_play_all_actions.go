// User story: As a Play UI user, the pre-call UI stays focused on dialing.
package ui_test

import (
	"testing"
)

func playAllActionsStories() []Story {
	return []Story{{
		Name: "202-play-all-actions",
		Run: func(_ testing.TB, page *Page) {
			page.GotoPlay("/")
			page.ExpectText("Start Video Call")
		},
	}}
}
