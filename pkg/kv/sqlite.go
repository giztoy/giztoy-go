package kv

import (
	"context"
	"database/sql"
	"iter"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// SQLite is a persistent Store backed by a single SQLite database file.
type SQLite struct {
	db   *sql.DB
	opts *Options
}

// NewSQLite opens (or creates) a SQLite-backed KV store in dir.
// The database file is named "kv.db" inside dir.
// Pass nil opts for defaults.
func NewSQLite(dir string, opts *Options) (*SQLite, error) {
	dbPath := filepath.Join(dir, "kv.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS kv (k BLOB PRIMARY KEY, v BLOB NOT NULL)`); err != nil {
		db.Close()
		return nil, err
	}
	return &SQLite{db: db, opts: opts}, nil
}

func (s *SQLite) Get(ctx context.Context, key Key) ([]byte, error) {
	var val []byte
	err := s.db.QueryRowContext(ctx, `SELECT v FROM kv WHERE k = ?`, s.opts.encode(key)).Scan(&val)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return val, err
}

func (s *SQLite) Set(ctx context.Context, key Key, value []byte) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO kv (k, v) VALUES (?, ?) ON CONFLICT(k) DO UPDATE SET v = excluded.v`,
		s.opts.encode(key), value)
	return err
}

func (s *SQLite) Delete(ctx context.Context, key Key) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM kv WHERE k = ?`, s.opts.encode(key))
	return err
}

func (s *SQLite) List(ctx context.Context, prefix Key) iter.Seq2[Entry, error] {
	return func(yield func(Entry, error) bool) {
		var rows *sql.Rows
		var err error

		p := s.opts.encode(prefix)
		if len(p) == 0 {
			rows, err = s.db.QueryContext(ctx, `SELECT k, v FROM kv ORDER BY k`)
		} else {
			lo := make([]byte, len(p)+1)
			copy(lo, p)
			lo[len(lo)-1] = s.opts.sep()
			hi := make([]byte, len(lo))
			copy(hi, lo)
			// Increment last byte to form an exclusive upper bound.
			// sep is always < 0xFF for valid separators, so this is safe.
			hi[len(hi)-1]++
			rows, err = s.db.QueryContext(ctx,
				`SELECT k, v FROM kv WHERE k >= ? AND k < ? ORDER BY k`, lo, hi)
		}
		if err != nil {
			yield(Entry{}, err)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var k, v []byte
			if err := rows.Scan(&k, &v); err != nil {
				if !yield(Entry{}, err) {
					return
				}
				continue
			}
			if !yield(Entry{Key: s.opts.decode(k), Value: v}, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(Entry{}, err)
		}
	}
}

func (s *SQLite) BatchSet(ctx context.Context, entries []Entry) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO kv (k, v) VALUES (?, ?) ON CONFLICT(k) DO UPDATE SET v = excluded.v`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		if _, err := stmt.ExecContext(ctx, s.opts.encode(e.Key), e.Value); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLite) BatchDelete(ctx context.Context, keys []Key) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `DELETE FROM kv WHERE k = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, k := range keys {
		if _, err := stmt.ExecContext(ctx, s.opts.encode(k)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLite) Close() error {
	return s.db.Close()
}

var _ Store = (*SQLite)(nil)
