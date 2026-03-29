// Package sql provides a relational store interface with table-based structured
// data. Each table has an auto-increment int64 row ID and typed columns.
// Records are represented as column-value maps.
//
// The package includes an in-memory implementation for testing. Production
// backends (e.g. SQLite) can implement the Store interface directly.
package sql

import (
	"context"
	"errors"
	"fmt"
	"iter"
)

// Sentinel errors.
var (
	ErrNotFound      = errors.New("sql: not found")
	ErrTableExists   = errors.New("sql: table already exists")
	ErrNoTable       = errors.New("sql: table does not exist")
	ErrUnknownColumn = errors.New("sql: unknown column")
	ErrNullValue     = errors.New("sql: null value for non-nullable column")
	ErrTypeMismatch  = errors.New("sql: type mismatch")
)

// RowID is the reserved column name for auto-generated row IDs.
// It is included in Record values returned by Get and List.
const RowID = "_rowid"

// ColumnType represents the data type of a table column.
type ColumnType int

const (
	TypeText    ColumnType = iota // Go type: string
	TypeInteger                   // Go type: int64 (int accepted on write, normalized to int64)
	TypeReal                      // Go type: float64 (float32 accepted on write, normalized to float64)
	TypeBlob                      // Go type: []byte
)

func (ct ColumnType) String() string {
	switch ct {
	case TypeText:
		return "Text"
	case TypeInteger:
		return "Integer"
	case TypeReal:
		return "Real"
	case TypeBlob:
		return "Blob"
	default:
		return fmt.Sprintf("ColumnType(%d)", int(ct))
	}
}

// Column describes a single column in a table schema.
type Column struct {
	Name     string
	Type     ColumnType
	Nullable bool
}

// Record is a row of column-value pairs. When returned from queries the
// special key RowID ("_rowid") holds the auto-generated int64 row ID.
type Record map[string]any

// Operator represents a comparison operator for filter conditions.
type Operator int

const (
	OpEq   Operator = iota // =
	OpNe                   // !=
	OpLt                   // <
	OpLe                   // <=
	OpGt                   // >
	OpGe                   // >=
	OpLike                 // LIKE (% and _ wildcards, case-insensitive)
)

// Condition is a single filter predicate on a column.
type Condition struct {
	Column string
	Op     Operator
	Value  any
}

// Conjunction controls how multiple conditions are combined.
type Conjunction int

const (
	And Conjunction = iota // all conditions must match
	Or                     // any condition may match
)

// OrderDir represents sort direction.
type OrderDir int

const (
	Asc  OrderDir = iota
	Desc
)

// OrderBy specifies a sort column and direction.
type OrderBy struct {
	Column string
	Dir    OrderDir
}

// ListOptions configures a List query.
type ListOptions struct {
	// Where specifies filter conditions. Empty means no filter.
	Where []Condition

	// Conjunction controls how Where conditions are combined.
	// Defaults to And (zero value).
	Conjunction Conjunction

	// OrderBy specifies sort order. Empty means insertion order.
	OrderBy []OrderBy

	// Limit is the maximum number of records to return. Zero means no limit.
	Limit int

	// Offset is the number of records to skip. Zero means no offset.
	Offset int
}

// Store is the interface for a SQL-based relational store.
type Store interface {
	// CreateTable creates a new table with the given column definitions.
	// Returns ErrTableExists if the table already exists.
	CreateTable(ctx context.Context, table string, columns []Column) error

	// DropTable removes a table and all its data.
	// Returns ErrNoTable if the table does not exist.
	DropTable(ctx context.Context, table string) error

	// Insert adds a new record and returns the auto-generated row ID.
	// The RowID field in record is ignored if present.
	Insert(ctx context.Context, table string, record Record) (int64, error)

	// Get retrieves a record by row ID. The returned Record includes RowID.
	// Returns ErrNotFound if the row does not exist.
	Get(ctx context.Context, table string, id int64) (Record, error)

	// Update modifies fields of an existing record. Only the fields present
	// in the record are updated; other fields are left unchanged. The RowID
	// field is ignored. Returns ErrNotFound if the row does not exist.
	Update(ctx context.Context, table string, id int64, fields Record) error

	// Delete removes a record by row ID. No error if the row does not exist.
	Delete(ctx context.Context, table string, id int64) error

	// List queries records from a table with optional filtering and ordering.
	// Pass nil opts to list all records in insertion order. Each returned
	// Record includes the RowID field.
	List(ctx context.Context, table string, opts *ListOptions) iter.Seq2[Record, error]

	// BatchInsert atomically inserts multiple records and returns their row IDs.
	BatchInsert(ctx context.Context, table string, records []Record) ([]int64, error)

	// BatchDelete atomically removes multiple records by row ID.
	BatchDelete(ctx context.Context, table string, ids []int64) error

	// Close releases any resources held by the store.
	Close() error
}
