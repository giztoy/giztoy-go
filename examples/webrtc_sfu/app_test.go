package main

import "testing"

func TestNormalizeRoomID(t *testing.T) {
	cases := map[string]string{
		"":              "demo",
		" Demo Room ":   "demoroom",
		"room-01":       "room-01",
		"%%%%":          "demo",
		"UPPER_lower12": "upper_lower12",
	}
	for input, want := range cases {
		if got := normalizeRoomID(input); got != want {
			t.Fatalf("normalizeRoomID(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeDisplayName(t *testing.T) {
	if got := normalizeDisplayName(""); got != "guest" {
		t.Fatalf("normalizeDisplayName(empty) = %q", got)
	}
	if got := normalizeDisplayName(" alice "); got != "alice" {
		t.Fatalf("normalizeDisplayName(trim) = %q", got)
	}
	long := "abcdefghijklmnopqrstuvwxyz0123456789"
	if got := normalizeDisplayName(long); len(got) != 32 {
		t.Fatalf("normalizeDisplayName(long) len = %d, want 32", len(got))
	}
}

func TestMakeRoomTrackID(t *testing.T) {
	got := makeRoomTrackID("peer-001", "stream-a", "video")
	if got != "peer-001/stream-a/video" {
		t.Fatalf("makeRoomTrackID() = %q", got)
	}
}

func TestRoomPeerSummariesLocked(t *testing.T) {
	r := &room{
		peers: map[string]*peer{
			"peer-002": {id: "peer-002", name: "b"},
			"peer-001": {id: "peer-001", name: "a"},
		},
	}
	got := r.peerSummariesLocked()
	if len(got) != 2 {
		t.Fatalf("peer count = %d, want 2", len(got))
	}
	if got[0].ID != "peer-001" || got[1].ID != "peer-002" {
		t.Fatalf("peer order = %+v", got)
	}
}

func TestRoomTrackListLocked(t *testing.T) {
	r := &room{
		tracks: map[string]*roomTrack{
			"b": {id: "b"},
			"a": {id: "a"},
		},
	}
	got := r.trackListLocked()
	if len(got) != 2 {
		t.Fatalf("track count = %d, want 2", len(got))
	}
	if got[0].id != "a" || got[1].id != "b" {
		t.Fatalf("track order = %q, %q", got[0].id, got[1].id)
	}
}

func TestRemovePeerDeletesEmptyRoom(t *testing.T) {
	a := &app{rooms: make(map[string]*room)}
	r := &room{
		app:   a,
		id:    "demo",
		peers: make(map[string]*peer),
	}
	a.rooms[r.id] = r

	p := &peer{id: "peer-001", room: r}
	r.peers[p.id] = p

	r.removePeer(p)

	if _, ok := a.rooms[r.id]; ok {
		t.Fatalf("room %q should be deleted when empty", r.id)
	}
}

func TestRemovePeerKeepsNonEmptyRoom(t *testing.T) {
	a := &app{rooms: make(map[string]*room)}
	r := &room{
		app:   a,
		id:    "demo",
		peers: make(map[string]*peer),
	}
	a.rooms[r.id] = r

	p1 := &peer{id: "peer-001", room: r}
	p2 := &peer{id: "peer-002", room: r}
	r.peers[p1.id] = p1
	r.peers[p2.id] = p2

	r.removePeer(p1)

	if _, ok := a.rooms[r.id]; !ok {
		t.Fatalf("room %q should remain while peers still exist", r.id)
	}
}
