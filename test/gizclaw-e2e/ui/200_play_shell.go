// User story: As a Play UI user, I can open the video-call shell and see the
// current call controls.
package ui_test

import (
	"testing"
)

func playShellStories() []Story {
	return []Story{{
		Name: "200-play-shell",
		Run: func(_ testing.TB, page *Page) {
			page.GotoPlay("/")
			page.ExpectText("WebRTC Play")
			page.ExpectText("RPC Log")
			page.ExpectText("Controls")
			page.ExpectText("Start Video Call")
			page.ExpectText("Dark")
			page.ExpectText("Light")
		},
	}}
}
