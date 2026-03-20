package kv

import (
	"context"
	"database/sql"
	"iter"

	_ "github.com/lib/pq"
)

// Postgres is a persistent Store backed by a PostgreSQL database.
type Postgres struct {
	db   *sql.DB
	opts *Options
}

// NewPostgres opens a PostgreSQL-backed KV store using the given DSN.
// The DSN follows the standard format: postgres://user:pass@host:port/dbname?sslmode=disable
// Pass nil opts for defaults.
func NewPostgres(dsn string, opts *Options) (*Postgres, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS kv (k BYTEA PRIMARY KEY, v BYTEA NOT NULL)`); err != nil {
		db.Close()
		return nil, err
	}
	return &Postgres{db: db, opts: opts}, nil
}

func (p *Postgres) Get(ctx context.Context, key Key) ([]byte, error) {
	var val []byte
	err := p.db.QueryRowContext(ctx, `SELECT v FROM kv WHERE k = $1`, p.opts.encode(key)).Scan(&val)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return val, err
}

func (p *Postgres) Set(ctx context.Context, key Key, value []byte) error {
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO kv (k, v) VALUES ($1, $2) ON CONFLICT(k) DO UPDATE SET v = EXCLUDED.v`,
		p.opts.encode(key), value)
	return err
}

func (p *Postgres) Delete(ctx context.Context, key Key) error {
	_, err := p.db.ExecContext(ctx, `DELETE FROM kv WHERE k = $1`, p.opts.encode(key))
	return err
}

func (p *Postgres) List(ctx context.Context, prefix Key) iter.Seq2[Entry, error] {
	return func(yield func(Entry, error) bool) {
		var rows *sql.Rows
		var err error

		pb := p.opts.encode(prefix)
		if len(pb) == 0 {
			rows, err = p.db.QueryContext(ctx, `SELECT k, v FROM kv ORDER BY k`)
		} else {
			lo := make([]byte, len(pb)+1)
			copy(lo, pb)
			lo[len(lo)-1] = p.opts.sep()
			hi := make([]byte, len(lo))
			copy(hi, lo)
			hi[len(hi)-1]++
			rows, err = p.db.QueryContext(ctx,
				`SELECT k, v FROM kv WHERE k >= $1 AND k < $2 ORDER BY k`, lo, hi)
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
			if !yield(Entry{Key: p.opts.decode(k), Value: v}, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(Entry{}, err)
		}
	}
}

func (p *Postgres) BatchSet(ctx context.Context, entries []Entry) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO kv (k, v) VALUES ($1, $2) ON CONFLICT(k) DO UPDATE SET v = EXCLUDED.v`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		if _, err := stmt.ExecContext(ctx, p.opts.encode(e.Key), e.Value); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (p *Postgres) BatchDelete(ctx context.Context, keys []Key) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `DELETE FROM kv WHERE k = $1`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, k := range keys {
		if _, err := stmt.ExecContext(ctx, p.opts.encode(k)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (p *Postgres) Close() error {
	return p.db.Close()
}

var _ Store = (*Postgres)(nil)
