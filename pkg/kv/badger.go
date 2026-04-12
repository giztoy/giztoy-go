package kv

import (
	"context"
	"iter"

	"github.com/dgraph-io/badger/v4"
)

// Badger is a persistent Store backed by BadgerDB.
type Badger struct {
	db   *badger.DB
	opts *Options
}

// NewBadger opens (or creates) a BadgerDB store at dir.
// Pass nil opts for defaults.
func NewBadger(dir string, opts *Options) (*Badger, error) {
	dbOpts := badger.DefaultOptions(dir).
		WithLogger(nil)
	db, err := badger.Open(dbOpts)
	if err != nil {
		return nil, err
	}
	return &Badger{db: db, opts: opts}, nil
}

func (b *Badger) Get(_ context.Context, key Key) ([]byte, error) {
	var val []byte
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(b.opts.encode(key))
		if err != nil {
			return err
		}
		val, err = item.ValueCopy(nil)
		return err
	})
	if err == badger.ErrKeyNotFound {
		return nil, ErrNotFound
	}
	return val, err
}

func (b *Badger) Set(_ context.Context, key Key, value []byte) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Set(b.opts.encode(key), value)
	})
}

func (b *Badger) Delete(_ context.Context, key Key) error {
	err := b.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(b.opts.encode(key))
	})
	if err == badger.ErrKeyNotFound {
		return nil
	}
	return err
}

func (b *Badger) List(_ context.Context, prefix Key) iter.Seq2[Entry, error] {
	return func(yield func(Entry, error) bool) {
		err := b.db.View(func(txn *badger.Txn) error {
			iterOpts := badger.DefaultIteratorOptions
			iterOpts.Prefix = b.listPrefix(prefix)
			it := txn.NewIterator(iterOpts)
			defer it.Close()

			for it.Seek(iterOpts.Prefix); it.Valid(); it.Next() {
				item := it.Item()
				val, err := item.ValueCopy(nil)
				if err != nil {
					if !yield(Entry{}, err) {
						return nil
					}
					continue
				}
				entry := Entry{
					Key:   b.opts.decode(item.KeyCopy(nil)),
					Value: val,
				}
				if !yield(entry, nil) {
					return nil
				}
			}
			return nil
		})
		if err != nil {
			yield(Entry{}, err)
		}
	}
}

func (b *Badger) BatchSet(_ context.Context, entries []Entry) error {
	return b.db.Update(func(txn *badger.Txn) error {
		for _, e := range entries {
			if err := txn.Set(b.opts.encode(e.Key), e.Value); err != nil {
				return err
			}
		}
		return nil
	})
}

func (b *Badger) BatchDelete(_ context.Context, keys []Key) error {
	return b.db.Update(func(txn *badger.Txn) error {
		for _, k := range keys {
			if err := txn.Delete(b.opts.encode(k)); err != nil && err != badger.ErrKeyNotFound {
				return err
			}
		}
		return nil
	})
}

func (b *Badger) Close() error {
	return b.db.Close()
}

// listPrefix returns the byte prefix for List iteration.
// For a non-empty prefix key it appends the separator so "a:b" doesn't match "a:bc".
func (b *Badger) listPrefix(prefix Key) []byte {
	p := b.opts.encode(prefix)
	if len(p) == 0 {
		return nil
	}
	return append(p, b.opts.sep())
}

// compile-time interface check
var _ Store = (*Badger)(nil)

// OpenBadger is an alias for NewBadger kept for discoverability.
// Deprecated: use NewBadger.
var OpenBadger = NewBadger

// NewBadgerWithOptions opens a BadgerDB store with custom badger.Options.
// This is useful for advanced tuning (e.g. in-memory mode, compression settings).
func NewBadgerWithOptions(dbOpts badger.Options, opts *Options) (*Badger, error) {
	dbOpts = dbOpts.WithLogger(nil)
	db, err := badger.Open(dbOpts)
	if err != nil {
		return nil, err
	}
	return &Badger{db: db, opts: opts}, nil
}

// NewBadgerInMemory creates an in-memory BadgerDB store (no disk persistence).
// Useful for integration tests that need a real Badger instance without temp dirs.
func NewBadgerInMemory(opts *Options) (*Badger, error) {
	dbOpts := badger.DefaultOptions("").
		WithInMemory(true).
		WithLogger(nil)
	return NewBadgerWithOptions(dbOpts, opts)
}

// RunGC triggers BadgerDB's value log garbage collection.
// discardRatio is the minimum fraction of entries that must be discarded
// for a rewrite to happen (typically 0.5).
// Returns nil if GC ran, badger.ErrNoRewrite if nothing to collect.
func (b *Badger) RunGC(discardRatio float64) error {
	return b.db.RunValueLogGC(discardRatio)
}

// Size returns the on-disk size in bytes as reported by BadgerDB.
func (b *Badger) Size() (lsm, vlog int64) {
	return b.db.Size()
}
