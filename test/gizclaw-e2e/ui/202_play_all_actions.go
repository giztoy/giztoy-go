// User story: As a Play UI user, I can start a WebRTC call, open the RPC data
// channel, and receive a server-info response through the local proxy.
package ui_test

import (
	"testing"
)

func playAllActionsStories() []Story {
	return []Story{{
		Name: "202-play-all-actions",
		Run: func(_ testing.TB, page *Page) {
			page.GotoPlay("/")
			page.ClickRole("button", "Start Video Call")
			page.ExpectText("Connected")
			page.ExpectText("rpc.open")
			page.ExpectText("server.info.get")
			page.ExpectText("rpc.response")
			page.ExpectText("build_commit")
		},
	}}
}
