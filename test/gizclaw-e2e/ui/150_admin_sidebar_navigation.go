// User story: As an admin operator, I can use the sidebar to move between the
// main Admin UI sections and land on each expected page.
package ui_test

import (
	"testing"
)

func adminSidebarNavigationStories() []Story {
	return []Story{{
		Name: "150-admin-sidebar-navigation",
		Run: func(_ testing.TB, page *Page) {
			page.GotoAdmin("/overview")
			for _, destination := range []struct {
				label   string
				heading string
				path    string
			}{
				{label: "Overview", heading: "Dashboard", path: "/overview"},
				{label: "Peers", heading: "Peers", path: "/peers"},
				{label: "Firmware", heading: "Depots", path: "/firmware"},
				{label: "Credentials", heading: "Credentials", path: "/providers/credentials"},
				{label: "MiniMax Tenants", heading: "MiniMax Tenants", path: "/providers/minimax-tenants"},
				{label: "Voices", heading: "Voices", path: "/ai/voices"},
				{label: "Workspace Templates", heading: "Workspace Templates", path: "/ai/workspace-templates"},
				{label: "Workspaces", heading: "Workspaces", path: "/ai/workspaces"},
			} {
				page.ClickNavigationLink(destination.label)
				page.ExpectURLSuffix(destination.path)
				page.ExpectText(destination.heading)
			}
		},
	}}
}
