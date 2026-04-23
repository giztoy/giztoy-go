package kv

import (
	"bytes"
	"context"
	"iter"
	"sort"
	"sync"
)

// Memory is an in-memory Store implementation backed by a sorted map.
// It is safe for concurrent use and intended primarily for testing.
type Memory struct {
	mu   sync.RWMutex
	data map[string][]byte
	opts *Options
}

// NewMemory creates a new in-memory Store.
// Pass nil for default options.
func NewMemory(opts *Options) *Memory {
	return &Memory{
		data: make(map[string][]byte),
		opts: opts,
	}
}

func (m *Memory) Get(ctx context.Context, key Key) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	k := string(m.opts.encode(key))
	m.mu.RLock()
	v, ok := m.data[k]
	m.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	cp := make([]byte, len(v))
	copy(cp, v)
	return cp, nil
}

func (m *Memory) Set(ctx context.Context, key Key, value []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	k := string(m.opts.encode(key))
	cp := make([]byte, len(value))
	copy(cp, value)
	m.mu.Lock()
	m.data[k] = cp
	m.mu.Unlock()
	return nil
}

func (m *Memory) Delete(ctx context.Context, key Key) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	k := string(m.opts.encode(key))
	m.mu.Lock()
	delete(m.data, k)
	m.mu.Unlock()
	return nil
}

func (m *Memory) List(ctx context.Context, prefix Key) iter.Seq2[Entry, error] {
	return func(yield func(Entry, error) bool) {
		if err := ctx.Err(); err != nil {
			yield(Entry{}, err)
			return
		}

		p := m.opts.encode(prefix)
		var prefixBytes []byte
		if len(p) > 0 {
			prefixBytes = append(p, m.opts.sep())
		}

		m.mu.RLock()
		type pair struct {
			key string
			val []byte
		}
		var matches []pair
		for k, v := range m.data {
			if len(prefixBytes) == 0 || bytes.HasPrefix([]byte(k), prefixBytes) {
				cp := make([]byte, len(v))
				copy(cp, v)
				matches = append(matches, pair{key: k, val: cp})
			}
		}
		m.mu.RUnlock()

		sort.Slice(matches, func(i, j int) bool {
			return matches[i].key < matches[j].key
		})

		for _, match := range matches {
			if err := ctx.Err(); err != nil {
				yield(Entry{}, err)
				return
			}
			entry := Entry{
				Key:   m.opts.decode([]byte(match.key)),
				Value: match.val,
			}
			if !yield(entry, nil) {
				return
			}
		}
	}
}

// ListAfter returns up to limit entries under the prefix subtree, strictly
// after the provided key.
func (m *Memory) ListAfter(ctx context.Context, prefix, after Key, limit int) ([]Entry, error) {
	if limit <= 0 {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, limit)
	for entry, err := range m.List(ctx, prefix) {
		if err != nil {
			return nil, err
		}
		if len(after) > 0 && entry.Key.String() <= after.String() {
			continue
		}
		entries = append(entries, entry)
		if len(entries) >= limit {
			break
		}
	}
	return entries, nil
}

func (m *Memory) BatchSet(ctx context.Context, entries []Entry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range entries {
		k := string(m.opts.encode(e.Key))
		cp := make([]byte, len(e.Value))
		copy(cp, e.Value)
		m.data[k] = cp
	}
	return nil
}

func (m *Memory) BatchDelete(ctx context.Context, keys []Key) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		k := string(m.opts.encode(key))
		delete(m.data, k)
	}
	return nil
}

func (m *Memory) Close() error {
	return nil
}

var _ Store = (*Memory)(nil)
