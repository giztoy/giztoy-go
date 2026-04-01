package memory

import (
	"context"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/kv"
)

func TestEmbedMetaModelMismatch(t *testing.T) {
	ctx := context.Background()
	store := kv.NewMemory(&kv.Options{Separator: testSep})

	baseEmb := newMockEmbedder()
	if _, err := NewHost(ctx, HostConfig{Store: store, Embedder: baseEmb, Separator: testSep}); err != nil {
		t.Fatalf("seed host with base embedder: %v", err)
	}

	differentModel := newMockEmbedder()
	differentModel.model = "another-model"
	if _, err := NewHost(ctx, HostConfig{Store: store, Embedder: differentModel, Separator: testSep}); err == nil {
		t.Fatal("expected model mismatch error, got nil")
	}
}

func TestEmbedMetaDimensionMismatch(t *testing.T) {
	ctx := context.Background()
	store := kv.NewMemory(&kv.Options{Separator: testSep})

	baseEmb := newMockEmbedder()
	if _, err := NewHost(ctx, HostConfig{Store: store, Embedder: baseEmb, Separator: testSep}); err != nil {
		t.Fatalf("seed host with base embedder: %v", err)
	}

	differentDim := newMockEmbedder()
	differentDim.dim = 4
	if _, err := NewHost(ctx, HostConfig{Store: store, Embedder: differentDim, Separator: testSep}); err == nil {
		t.Fatal("expected dimension mismatch error, got nil")
	}
}

func TestOpenWithEmbedderModelMismatchAgainstHost(t *testing.T) {
	ctx := context.Background()
	store := kv.NewMemory(&kv.Options{Separator: testSep})

	hostEmb := newMockEmbedder()
	host, err := NewHost(ctx, HostConfig{Store: store, Embedder: hostEmb, Separator: testSep})
	if err != nil {
		t.Fatalf("new host: %v", err)
	}

	mismatch := newMockEmbedder()
	mismatch.model = "other-model"
	if _, err := host.Open("persona-a", WithEmbedder(mismatch)); err == nil {
		t.Fatal("expected model mismatch on WithEmbedder override, got nil")
	}
}

func TestOpenWithEmbedderChecksPersistedMetaWhenHostEmbedderNil(t *testing.T) {
	ctx := context.Background()
	store := kv.NewMemory(&kv.Options{Separator: testSep})

	seedEmb := newMockEmbedder()
	seedHost, err := NewHost(ctx, HostConfig{Store: store, Embedder: seedEmb, Separator: testSep})
	if err != nil {
		t.Fatalf("seed host: %v", err)
	}
	if _, err := seedHost.Open("seed"); err != nil {
		t.Fatalf("open seed persona: %v", err)
	}

	hostNoDefault, err := NewHost(ctx, HostConfig{Store: store, Separator: testSep})
	if err != nil {
		t.Fatalf("host without default embedder: %v", err)
	}

	mismatch := newMockEmbedder()
	mismatch.model = "other-model"
	if _, err := hostNoDefault.Open("persona-b", WithEmbedder(mismatch)); err == nil {
		t.Fatal("expected persisted embed meta check to reject mismatched per-persona embedder")
	}
}
