package ui_test

import "testing"

func TestUIStories(t *testing.T) {
	RunStories(t, allStories())
}

func allStories() []Story {
	var stories []Story
	for _, group := range [][]Story{
		adminDashboardStories(),
		adminLegacyHashRouteStories(),
		adminPeersListStories(),
		adminPeerDetailStories(),
		adminPeerActionsStories(),
		adminFirmwareListStories(),
		adminFirmwareUploadStories(),
		adminDepotDetailStories(),
		adminDepotActionsStories(),
		adminChannelDetailStories(),
		adminCredentialsListStories(),
		adminMiniMaxTenantsListStories(),
		adminVoicesListStories(),
		adminWorkspaceTemplatesListStories(),
		adminWorkspacesListStories(),
		adminSidebarNavigationStories(),
		playShellStories(),
		playActionsStories(),
		playAllActionsStories(),
		playActionErrorsStories(),
		realServiceSmokeStories(),
	} {
		stories = append(stories, group...)
	}
	return stories
}
