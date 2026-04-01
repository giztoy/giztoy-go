package recall

import (
	"context"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/pkg/graph"
	"github.com/giztoy/giztoy-go/pkg/kv"
)

func TestBucketDurationAndBucketForSpan(t *testing.T) {
	durationCases := []struct {
		bucket Bucket
		want   time.Duration
	}{
		{Bucket1H, time.Hour},
		{Bucket1D, 24 * time.Hour},
		{Bucket1W, 7 * 24 * time.Hour},
		{Bucket1M, 30 * 24 * time.Hour},
		{Bucket3M, 90 * 24 * time.Hour},
		{Bucket6M, 180 * 24 * time.Hour},
		{Bucket1Y, 365 * 24 * time.Hour},
		{BucketLT, 0},
		{Bucket("unknown"), 0},
	}

	for _, tc := range durationCases {
		if got := BucketDuration(tc.bucket); got != tc.want {
			t.Fatalf("BucketDuration(%s)=%v, want %v", tc.bucket, got, tc.want)
		}
	}

	spanCases := []struct {
		span time.Duration
		want Bucket
	}{
		{30 * time.Minute, Bucket1H},
		{1 * time.Hour, Bucket1H},
		{12 * time.Hour, Bucket1D},
		{5 * 24 * time.Hour, Bucket1W},
		{20 * 24 * time.Hour, Bucket1M},
		{100 * 24 * time.Hour, Bucket6M},
		{300 * 24 * time.Hour, Bucket1Y},
		{800 * 24 * time.Hour, BucketLT},
	}
	for _, tc := range spanCases {
		if got := BucketForSpan(tc.span); got != tc.want {
			t.Fatalf("BucketForSpan(%v)=%s, want %s", tc.span, got, tc.want)
		}
	}
}

func TestBucketSegmentsAndBucketStats(t *testing.T) {
	idx := newTestIndexNoVec(t)
	ctx := context.Background()

	segments := []Segment{
		{ID: "h-1", Summary: "a", Timestamp: 100, Bucket: Bucket1H},
		{ID: "h-2", Summary: "hello", Timestamp: 200, Bucket: Bucket1H},
		{ID: "d-1", Summary: "daily", Timestamp: 300, Bucket: Bucket1D},
	}
	for _, seg := range segments {
		if err := idx.StoreSegment(ctx, seg); err != nil {
			t.Fatalf("StoreSegment(%s): %v", seg.ID, err)
		}
	}

	// Add one malformed entry under 1h prefix; BucketSegments/BucketStats should skip it.
	if err := idx.store.Set(ctx, segmentKey(idx.prefix, Bucket1H, 250), []byte("not-msgpack")); err != nil {
		t.Fatalf("inject malformed segment: %v", err)
	}

	hourSegments, err := idx.BucketSegments(ctx, Bucket1H)
	if err != nil {
		t.Fatalf("BucketSegments(1h): %v", err)
	}
	if len(hourSegments) != 2 {
		t.Fatalf("len(hourSegments)=%d, want 2", len(hourSegments))
	}
	if hourSegments[0].ID != "h-1" || hourSegments[1].ID != "h-2" {
		t.Fatalf("unexpected 1h segment order: %+v", hourSegments)
	}

	count, chars, err := idx.BucketStats(ctx, Bucket1H)
	if err != nil {
		t.Fatalf("BucketStats(1h): %v", err)
	}
	if count != 2 {
		t.Fatalf("BucketStats count=%d, want 2", count)
	}
	if chars != len("a")+len("hello") {
		t.Fatalf("BucketStats chars=%d, want %d", chars, len("a")+len("hello"))
	}

	empty, err := idx.BucketSegments(ctx, Bucket1W)
	if err != nil {
		t.Fatalf("BucketSegments(1w): %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected empty 1w bucket, got %d segments", len(empty))
	}
}

func TestGetSegmentHandlesOrphanSidAndCorruptData(t *testing.T) {
	idx := newTestIndexNoVec(t)
	ctx := context.Background()

	// Orphan sid: sid exists but segment data key missing.
	if err := idx.store.Set(ctx, sidKey(idx.prefix, "orphan"), sidValue(Bucket1H, 123)); err != nil {
		t.Fatalf("set sid orphan: %v", err)
	}
	got, err := idx.GetSegment(ctx, "orphan")
	if err != nil {
		t.Fatalf("GetSegment(orphan): %v", err)
	}
	if got != nil {
		t.Fatalf("GetSegment(orphan)=%+v, want nil", got)
	}

	// Corrupt payload: sid + segment key exist, but data cannot be unmarshaled.
	if err := idx.store.Set(ctx, sidKey(idx.prefix, "bad"), sidValue(Bucket1H, 456)); err != nil {
		t.Fatalf("set sid bad: %v", err)
	}
	if err := idx.store.Set(ctx, segmentKey(idx.prefix, Bucket1H, 456), []byte{0xff, 0x00, 0x01}); err != nil {
		t.Fatalf("set bad payload: %v", err)
	}
	got, err = idx.GetSegment(ctx, "bad")
	if err == nil {
		t.Fatal("expected unmarshal error for corrupt segment payload")
	}
	if got != nil {
		t.Fatalf("GetSegment(bad)=%+v, want nil", got)
	}
}

func TestNewIndexNilStorePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when NewIndex called with nil store")
		}
	}()

	_ = NewIndex(IndexConfig{Store: nil})
}

func TestNewIndexSeparatorAffectsGraphLabelValidation(t *testing.T) {
	ctx := context.Background()

	// Default separator ':' should reject labels containing ':'.
	idxDefault := NewIndex(IndexConfig{Store: kv.NewMemory(nil), Prefix: kv.Key{"default"}})
	if err := idxDefault.Graph().SetEntity(ctx, graph.Entity{Label: "person:alice"}); err == nil {
		t.Fatal("expected default separator graph to reject label containing ':'")
	}

	// Custom separator should allow colon-namespaced labels.
	const sep byte = 0x1F
	idxCustom := NewIndex(IndexConfig{
		Store:     kv.NewMemory(&kv.Options{Separator: sep}),
		Prefix:    kv.Key{"custom"},
		Separator: sep,
	})
	if err := idxCustom.Graph().SetEntity(ctx, graph.Entity{Label: "person:alice"}); err != nil {
		t.Fatalf("custom separator graph should allow ':' labels: %v", err)
	}
}
