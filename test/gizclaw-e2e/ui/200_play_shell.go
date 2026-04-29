// User story: As a Play UI user, I can open the shell and see the local
// WebRTC dial button before a call starts.
package ui_test

import (
	"testing"
)

func playShellStories() []Story {
	return []Story{{
		Name: "200-play-shell",
		Run: func(_ testing.TB, page *Page) {
			page.GotoPlay("/")
			page.ExpectText("Start Video Call")
		},
	}}
}
