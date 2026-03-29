package sql

import (
	"cmp"
	"context"
	"fmt"
	"iter"
	"slices"
	"strings"
	"sync"
)

// Memory is an in-memory Store implementation backed by maps.
// It is safe for concurrent use and intended primarily for testing.
type Memory struct {
	mu     sync.RWMutex
	tables map[string]*memTable
}

type memTable struct {
	columns []Column
	rows    map[int64]Record
	nextID  int64
}

// NewMemory creates a new in-memory Store.
func NewMemory() *Memory {
	return &Memory{
		tables: make(map[string]*memTable),
	}
}

func (m *Memory) CreateTable(_ context.Context, table string, columns []Column) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tables[table]; ok {
		return ErrTableExists
	}
	cols := make([]Column, len(columns))
	copy(cols, columns)
	m.tables[table] = &memTable{
		columns: cols,
		rows:    make(map[int64]Record),
		nextID:  1,
	}
	return nil
}

func (m *Memory) DropTable(_ context.Context, table string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tables[table]; !ok {
		return ErrNoTable
	}
	delete(m.tables, table)
	return nil
}

func (m *Memory) Insert(_ context.Context, table string, record Record) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tables[table]
	if !ok {
		return 0, ErrNoTable
	}
	if err := t.validateInsert(record); err != nil {
		return 0, err
	}
	id := t.nextID
	t.nextID++
	t.rows[id] = copyRecord(record, id)
	return id, nil
}

func (m *Memory) Get(_ context.Context, table string, id int64) (Record, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tables[table]
	if !ok {
		return nil, ErrNoTable
	}
	r, ok := t.rows[id]
	if !ok {
		return nil, ErrNotFound
	}
	return copyRecord(r, id), nil
}

func (m *Memory) Update(_ context.Context, table string, id int64, fields Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tables[table]
	if !ok {
		return ErrNoTable
	}
	existing, ok := t.rows[id]
	if !ok {
		return ErrNotFound
	}
	if err := t.validateUpdate(fields); err != nil {
		return err
	}
	for k, v := range fields {
		if k == RowID {
			continue
		}
		switch tv := v.(type) {
		case []byte:
			dup := make([]byte, len(tv))
			copy(dup, tv)
			existing[k] = dup
		case int:
			existing[k] = int64(tv)
		case float32:
			existing[k] = float64(tv)
		default:
			existing[k] = v
		}
	}
	return nil
}

func (m *Memory) Delete(_ context.Context, table string, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tables[table]
	if !ok {
		return ErrNoTable
	}
	delete(t.rows, id)
	return nil
}

func (m *Memory) List(_ context.Context, table string, opts *ListOptions) iter.Seq2[Record, error] {
	m.mu.RLock()
	t, ok := m.tables[table]
	if !ok {
		m.mu.RUnlock()
		return func(yield func(Record, error) bool) {
			yield(nil, ErrNoTable)
		}
	}

	// Snapshot rows under read lock.
	rows := make([]Record, 0, len(t.rows))
	for id, r := range t.rows {
		rows = append(rows, copyRecord(r, id))
	}
	m.mu.RUnlock()

	// Filter.
	if opts != nil && len(opts.Where) > 0 {
		rows = filterRecords(rows, opts.Where, opts.Conjunction)
	}

	// Sort: default by RowID ascending (insertion order).
	if opts != nil && len(opts.OrderBy) > 0 {
		sortRecords(rows, opts.OrderBy)
	} else {
		slices.SortStableFunc(rows, func(a, b Record) int {
			return cmp.Compare(a[RowID].(int64), b[RowID].(int64))
		})
	}

	// Offset and limit.
	if opts != nil && opts.Offset > 0 {
		if opts.Offset >= len(rows) {
			rows = nil
		} else {
			rows = rows[opts.Offset:]
		}
	}
	if opts != nil && opts.Limit > 0 && opts.Limit < len(rows) {
		rows = rows[:opts.Limit]
	}

	return func(yield func(Record, error) bool) {
		for _, r := range rows {
			if !yield(r, nil) {
				return
			}
		}
	}
}

func (m *Memory) BatchInsert(_ context.Context, table string, records []Record) ([]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tables[table]
	if !ok {
		return nil, ErrNoTable
	}
	for _, r := range records {
		if err := t.validateInsert(r); err != nil {
			return nil, err
		}
	}
	ids := make([]int64, len(records))
	for i, r := range records {
		id := t.nextID
		t.nextID++
		t.rows[id] = copyRecord(r, id)
		ids[i] = id
	}
	return ids, nil
}

func (m *Memory) BatchDelete(_ context.Context, table string, ids []int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tables[table]
	if !ok {
		return ErrNoTable
	}
	for _, id := range ids {
		delete(t.rows, id)
	}
	return nil
}

func (m *Memory) Close() error {
	return nil
}

// compile-time interface check
var _ Store = (*Memory)(nil)

// --- helpers ---

// copyRecord returns a copy of r with the RowID field set.
// []byte values are deep-copied and numeric types are normalized
// (int→int64, float32→float64) to ensure caller isolation and
// consistent types on read.
func copyRecord(r Record, id int64) Record {
	cp := make(Record, len(r)+1)
	for k, v := range r {
		if k == RowID {
			continue
		}
		switch tv := v.(type) {
		case []byte:
			dup := make([]byte, len(tv))
			copy(dup, tv)
			cp[k] = dup
		case int:
			cp[k] = int64(tv)
		case float32:
			cp[k] = float64(tv)
		default:
			cp[k] = v
		}
	}
	cp[RowID] = id
	return cp
}

func (t *memTable) columnByName(name string) (Column, bool) {
	for _, c := range t.columns {
		if c.Name == name {
			return c, true
		}
	}
	return Column{}, false
}

func checkType(col Column, v any) bool {
	switch col.Type {
	case TypeText:
		_, ok := v.(string)
		return ok
	case TypeInteger:
		switch v.(type) {
		case int64, int:
			return true
		}
		return false
	case TypeReal:
		switch v.(type) {
		case float64, float32:
			return true
		}
		return false
	case TypeBlob:
		_, ok := v.([]byte)
		return ok
	}
	return false
}

func (t *memTable) validateInsert(record Record) error {
	for k, v := range record {
		if k == RowID {
			continue
		}
		col, ok := t.columnByName(k)
		if !ok {
			return fmt.Errorf("%w: %s", ErrUnknownColumn, k)
		}
		if v != nil && !checkType(col, v) {
			return fmt.Errorf("%w: column %s expects %v", ErrTypeMismatch, k, col.Type)
		}
	}
	for _, col := range t.columns {
		if col.Nullable {
			continue
		}
		v, ok := record[col.Name]
		if !ok || v == nil {
			return fmt.Errorf("%w: %s", ErrNullValue, col.Name)
		}
	}
	return nil
}

func (t *memTable) validateUpdate(fields Record) error {
	for k, v := range fields {
		if k == RowID {
			continue
		}
		col, ok := t.columnByName(k)
		if !ok {
			return fmt.Errorf("%w: %s", ErrUnknownColumn, k)
		}
		if v == nil {
			if !col.Nullable {
				return fmt.Errorf("%w: %s", ErrNullValue, k)
			}
			continue
		}
		if !checkType(col, v) {
			return fmt.Errorf("%w: column %s expects %v", ErrTypeMismatch, k, col.Type)
		}
	}
	return nil
}

func filterRecords(rows []Record, conditions []Condition, conj Conjunction) []Record {
	result := make([]Record, 0, len(rows))
	for _, r := range rows {
		if matchRecord(r, conditions, conj) {
			result = append(result, r)
		}
	}
	return result
}

func matchRecord(r Record, conditions []Condition, conj Conjunction) bool {
	if conj == Or {
		for _, c := range conditions {
			if evalCondition(r, c) {
				return true
			}
		}
		return false
	}
	for _, c := range conditions {
		if !evalCondition(r, c) {
			return false
		}
	}
	return true
}

func evalCondition(r Record, c Condition) bool {
	v, ok := r[c.Column]
	if !ok || v == nil {
		switch c.Op {
		case OpEq:
			return c.Value == nil
		case OpNe:
			return c.Value != nil
		default:
			return false
		}
	}
	if c.Value == nil {
		switch c.Op {
		case OpEq:
			return false
		case OpNe:
			return true
		default:
			return false
		}
	}

	if c.Op == OpLike {
		return matchLike(fmt.Sprint(v), fmt.Sprint(c.Value))
	}

	ord, comparable := compareAny(v, c.Value)
	if !comparable {
		return false
	}
	switch c.Op {
	case OpEq:
		return ord == 0
	case OpNe:
		return ord != 0
	case OpLt:
		return ord < 0
	case OpLe:
		return ord <= 0
	case OpGt:
		return ord > 0
	case OpGe:
		return ord >= 0
	}
	return false
}

// compareAny returns (ordering, true) if a and b are orderable, (0, false) otherwise.
// a comes from stored records (always normalized: string, int64, float64).
// b comes from filter values (may be int, float32, etc.).
func compareAny(a, b any) (int, bool) {
	switch av := a.(type) {
	case string:
		if bv, ok := b.(string); ok {
			return cmp.Compare(av, bv), true
		}
	case int64:
		switch bv := b.(type) {
		case int64:
			return cmp.Compare(av, bv), true
		case int:
			return cmp.Compare(av, int64(bv)), true
		case float64:
			return cmp.Compare(float64(av), bv), true
		case float32:
			return cmp.Compare(float64(av), float64(bv)), true
		}
	case float64:
		switch bv := b.(type) {
		case float64:
			return cmp.Compare(av, bv), true
		case float32:
			return cmp.Compare(av, float64(bv)), true
		case int64:
			return cmp.Compare(av, float64(bv)), true
		case int:
			return cmp.Compare(av, float64(bv)), true
		}
	}
	return 0, false
}

// matchLike implements SQL LIKE with % and _ wildcards.
// It operates on runes for correct UTF-8 handling and uses an
// iterative algorithm with O(n·m) worst-case complexity.
func matchLike(s, pattern string) bool {
	sr := []rune(strings.ToLower(s))
	pr := []rune(strings.ToLower(pattern))
	si, pi := 0, 0
	starPI, starSI := -1, -1

	for si < len(sr) {
		switch {
		case pi < len(pr) && (pr[pi] == '_' || pr[pi] == sr[si]):
			si++
			pi++
		case pi < len(pr) && pr[pi] == '%':
			starPI = pi
			starSI = si
			pi++
		case starPI >= 0:
			starSI++
			si = starSI
			pi = starPI + 1
		default:
			return false
		}
	}

	for pi < len(pr) && pr[pi] == '%' {
		pi++
	}
	return pi == len(pr)
}

func sortRecords(rows []Record, orderBy []OrderBy) {
	slices.SortStableFunc(rows, func(a, b Record) int {
		for _, ob := range orderBy {
			av, aOk := a[ob.Column]
			bv, bOk := b[ob.Column]
			aNil := !aOk || av == nil
			bNil := !bOk || bv == nil

			if aNil && bNil {
				continue
			}
			if aNil {
				return 1 // NULLS LAST
			}
			if bNil {
				return -1
			}

			c, ok := compareAny(av, bv)
			if !ok || c == 0 {
				continue
			}
			if ob.Dir == Desc {
				c = -c
			}
			return c
		}
		return 0
	})
}
