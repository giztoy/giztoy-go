package stores

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/haivivi/giztoy/go/pkg/vecstore"
)

const hnswIndexFilename = "index.hnsw"

// persistentHNSW wraps vecstore.HNSW with on-disk save/load.
type persistentHNSW struct {
	mu    sync.Mutex
	path  string
	index *vecstore.HNSW
	dirty bool
}

func openPersistentHNSW(path string, cfg vecstore.HNSWConfig) (*persistentHNSW, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("stores: hnsw mkdir: %w", err)
	}

	idx, err := loadOrCreateHNSW(path, cfg)
	if err != nil {
		return nil, err
	}
	return &persistentHNSW{path: path, index: idx}, nil
}

func loadOrCreateHNSW(path string, cfg vecstore.HNSWConfig) (*vecstore.HNSW, error) {
	f, err := os.Open(path)
	if err == nil {
		defer f.Close()
		idx, loadErr := vecstore.LoadHNSW(f)
		if loadErr != nil {
			return nil, fmt.Errorf("stores: hnsw load %q: %w", path, loadErr)
		}
		if got := idx.Config().Dim; got != cfg.Dim {
			return nil, fmt.Errorf("stores: hnsw load %q: dimension mismatch: file=%d config=%d", path, got, cfg.Dim)
		}
		return idx, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stores: hnsw open %q: %w", path, err)
	}
	return vecstore.NewHNSW(cfg), nil
}

func (h *persistentHNSW) Insert(id string, vector []float32) error {
	if err := h.index.Insert(id, vector); err != nil {
		return err
	}
	h.markDirty()
	return nil
}

func (h *persistentHNSW) BatchInsert(ids []string, vectors [][]float32) error {
	if err := h.index.BatchInsert(ids, vectors); err != nil {
		return err
	}
	h.markDirty()
	return nil
}

func (h *persistentHNSW) Search(query []float32, topK int) ([]vecstore.Match, error) {
	return h.index.Search(query, topK)
}

func (h *persistentHNSW) Delete(id string) error {
	if err := h.index.Delete(id); err != nil {
		return err
	}
	h.markDirty()
	return nil
}

func (h *persistentHNSW) Len() int {
	return h.index.Len()
}

func (h *persistentHNSW) Flush() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.flushLocked()
}

func (h *persistentHNSW) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.flushLocked(); err != nil {
		return err
	}
	return h.index.Close()
}

func (h *persistentHNSW) markDirty() {
	h.mu.Lock()
	h.dirty = true
	h.mu.Unlock()
}

func (h *persistentHNSW) flushLocked() error {
	if !h.dirty {
		return nil
	}
	if err := saveHNSW(h.path, h.index); err != nil {
		return err
	}
	h.dirty = false
	return nil
}

func saveHNSW(path string, idx *vecstore.HNSW) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("stores: hnsw create %q: %w", tmp, err)
	}
	defer func() {
		_ = f.Close()
	}()

	if err := idx.Save(f); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("stores: hnsw save %q: %w", path, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("stores: hnsw close %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("stores: hnsw rename %q: %w", path, err)
	}
	return nil
}
