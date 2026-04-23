package client

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw"
)

func TestProbeServerPublicReadyNilClient(t *testing.T) {
	err := probeServerPublicReady(nil)
	if err == nil {
		t.Fatal("probeServerPublicReady should fail for nil client")
	}
	if !strings.Contains(err.Error(), "nil client") {
		t.Fatalf("probeServerPublicReady error = %v", err)
	}
}

func TestProbeServerPublicReadyRequiresConnection(t *testing.T) {
	err := probeServerPublicReady(&gizclaw.Client{})
	if err == nil {
		t.Fatal("probeServerPublicReady should fail without connection")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("probeServerPublicReady error = %v", err)
	}
}

func TestCollectAllPagesAggregatesUntilDone(t *testing.T) {
	var seen []string
	items, err := collectAllPages(func(cursor *adminservice.Cursor, limit *adminservice.Limit) (pagedItems[string], error) {
		if limit == nil || *limit != 200 {
			t.Fatalf("limit = %v, want 200", limit)
		}
		label := "<nil>"
		if cursor != nil {
			label = string(*cursor)
		}
		seen = append(seen, label)
		switch label {
		case "<nil>":
			next := "page-1"
			return pagedItems[string]{HasNext: true, Items: []string{"a", "b"}, NextCursor: &next}, nil
		case "page-1":
			next := "page-2"
			return pagedItems[string]{HasNext: true, Items: []string{"c"}, NextCursor: &next}, nil
		case "page-2":
			return pagedItems[string]{HasNext: false, Items: []string{"d"}}, nil
		default:
			t.Fatalf("unexpected cursor %q", label)
			return pagedItems[string]{}, nil
		}
	})
	if err != nil {
		t.Fatalf("collectAllPages error = %v", err)
	}
	if !slices.Equal(seen, []string{"<nil>", "page-1", "page-2"}) {
		t.Fatalf("seen cursors = %v", seen)
	}
	if !slices.Equal(items, []string{"a", "b", "c", "d"}) {
		t.Fatalf("items = %v", items)
	}
}

func TestCollectAllPagesStopsOnMissingNextCursor(t *testing.T) {
	calls := 0
	items, err := collectAllPages(func(cursor *adminservice.Cursor, limit *adminservice.Limit) (pagedItems[string], error) {
		calls++
		return pagedItems[string]{HasNext: true, Items: []string{"a"}}, nil
	})
	if err != nil {
		t.Fatalf("collectAllPages error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	if !slices.Equal(items, []string{"a"}) {
		t.Fatalf("items = %v", items)
	}
}

func TestCollectAllPagesPropagatesErrors(t *testing.T) {
	want := errors.New("boom")
	_, err := collectAllPages(func(cursor *adminservice.Cursor, limit *adminservice.Limit) (pagedItems[string], error) {
		return pagedItems[string]{}, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}
