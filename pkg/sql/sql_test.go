package sql_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/sql"
)

func newTestStore(t *testing.T) sql.Store {
	t.Helper()
	s := sql.NewMemory()
	t.Cleanup(func() { s.Close() })
	return s
}

func createUsersTable(t *testing.T, s sql.Store) {
	t.Helper()
	ctx := context.Background()
	err := s.CreateTable(ctx, "users", []sql.Column{
		{Name: "name", Type: sql.TypeText},
		{Name: "age", Type: sql.TypeInteger, Nullable: true},
		{Name: "score", Type: sql.TypeReal, Nullable: true},
	})
	if err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
}

func TestCreateDropTable(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	err := s.CreateTable(ctx, "t1", []sql.Column{
		{Name: "col1", Type: sql.TypeText},
	})
	if err != nil {
		t.Fatalf("CreateTable: %v", err)
	}

	// Duplicate creation should fail.
	err = s.CreateTable(ctx, "t1", nil)
	if !errors.Is(err, sql.ErrTableExists) {
		t.Fatalf("expected ErrTableExists, got %v", err)
	}

	// Drop and re-create should work.
	if err := s.DropTable(ctx, "t1"); err != nil {
		t.Fatalf("DropTable: %v", err)
	}
	err = s.CreateTable(ctx, "t1", nil)
	if err != nil {
		t.Fatalf("CreateTable after drop: %v", err)
	}

	// Drop non-existent table.
	err = s.DropTable(ctx, "no_such_table")
	if !errors.Is(err, sql.ErrNoTable) {
		t.Fatalf("expected ErrNoTable, got %v", err)
	}
}

func TestInsertGetUpdateDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	// Insert.
	id, err := s.Insert(ctx, "users", sql.Record{
		"name": "Alice", "age": int64(30), "score": 95.5,
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if id <= 0 {
		t.Fatalf("Insert returned non-positive id: %d", id)
	}

	// Get.
	r, err := s.Get(ctx, "users", id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if r["name"] != "Alice" || r["age"] != int64(30) || r["score"] != 95.5 {
		t.Fatalf("Get = %v, unexpected values", r)
	}
	if r[sql.RowID] != id {
		t.Fatalf("RowID = %v, want %d", r[sql.RowID], id)
	}

	// Get non-existent.
	_, err = s.Get(ctx, "users", 9999)
	if !errors.Is(err, sql.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// Update partial fields.
	err = s.Update(ctx, "users", id, sql.Record{"age": int64(31)})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	r, _ = s.Get(ctx, "users", id)
	if r["age"] != int64(31) {
		t.Fatalf("age after Update = %v, want 31", r["age"])
	}
	if r["name"] != "Alice" {
		t.Fatalf("name after Update = %v, want Alice (should be unchanged)", r["name"])
	}

	// Update non-existent row.
	err = s.Update(ctx, "users", 9999, sql.Record{"age": int64(1)})
	if !errors.Is(err, sql.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// Delete.
	if err := s.Delete(ctx, "users", id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = s.Get(ctx, "users", id)
	if !errors.Is(err, sql.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}

	// Delete non-existent row should not error.
	if err := s.Delete(ctx, "users", 9999); err != nil {
		t.Fatalf("Delete non-existent: %v", err)
	}
}

func TestNoTableErrors(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	_, err := s.Insert(ctx, "nope", sql.Record{"a": "b"})
	if !errors.Is(err, sql.ErrNoTable) {
		t.Fatalf("Insert into missing table: expected ErrNoTable, got %v", err)
	}

	_, err = s.Get(ctx, "nope", 1)
	if !errors.Is(err, sql.ErrNoTable) {
		t.Fatalf("Get from missing table: expected ErrNoTable, got %v", err)
	}

	err = s.Update(ctx, "nope", 1, sql.Record{"a": "b"})
	if !errors.Is(err, sql.ErrNoTable) {
		t.Fatalf("Update missing table: expected ErrNoTable, got %v", err)
	}

	err = s.Delete(ctx, "nope", 1)
	if !errors.Is(err, sql.ErrNoTable) {
		t.Fatalf("Delete from missing table: expected ErrNoTable, got %v", err)
	}

	for _, err := range s.List(ctx, "nope", nil) {
		if !errors.Is(err, sql.ErrNoTable) {
			t.Fatalf("List from missing table: expected ErrNoTable, got %v", err)
		}
	}

	_, err = s.BatchInsert(ctx, "nope", nil)
	if !errors.Is(err, sql.ErrNoTable) {
		t.Fatalf("BatchInsert missing table: expected ErrNoTable, got %v", err)
	}

	err = s.BatchDelete(ctx, "nope", nil)
	if !errors.Is(err, sql.ErrNoTable) {
		t.Fatalf("BatchDelete missing table: expected ErrNoTable, got %v", err)
	}
}

func TestListBasic(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	records := []sql.Record{
		{"name": "Alice", "age": int64(30)},
		{"name": "Bob", "age": int64(25)},
		{"name": "Charlie", "age": int64(35)},
	}
	if _, err := s.BatchInsert(ctx, "users", records); err != nil {
		t.Fatalf("BatchInsert: %v", err)
	}

	// List all, default order (by RowID).
	var names []string
	for r, err := range s.List(ctx, "users", nil) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	want := []string{"Alice", "Bob", "Charlie"}
	if !slices.Equal(names, want) {
		t.Fatalf("List = %v, want %v", names, want)
	}
}

func TestListFilterAnd(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	records := []sql.Record{
		{"name": "Alice", "age": int64(30), "score": 90.0},
		{"name": "Bob", "age": int64(25), "score": 85.0},
		{"name": "Charlie", "age": int64(35), "score": 92.0},
		{"name": "Diana", "age": int64(28), "score": 88.0},
	}
	if _, err := s.BatchInsert(ctx, "users", records); err != nil {
		t.Fatalf("BatchInsert: %v", err)
	}

	// age >= 28 AND score > 88.0
	var names []string
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{
			{Column: "age", Op: sql.OpGe, Value: int64(28)},
			{Column: "score", Op: sql.OpGt, Value: 88.0},
		},
		Conjunction: sql.And,
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	want := []string{"Alice", "Charlie"}
	if !slices.Equal(names, want) {
		t.Fatalf("List AND filter = %v, want %v", names, want)
	}
}

func TestListFilterOr(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	records := []sql.Record{
		{"name": "Alice", "age": int64(30)},
		{"name": "Bob", "age": int64(25)},
		{"name": "Charlie", "age": int64(35)},
	}
	if _, err := s.BatchInsert(ctx, "users", records); err != nil {
		t.Fatalf("BatchInsert: %v", err)
	}

	// age == 25 OR age == 35
	var names []string
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{
			{Column: "age", Op: sql.OpEq, Value: int64(25)},
			{Column: "age", Op: sql.OpEq, Value: int64(35)},
		},
		Conjunction: sql.Or,
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	want := []string{"Bob", "Charlie"}
	if !slices.Equal(names, want) {
		t.Fatalf("List OR filter = %v, want %v", names, want)
	}
}

func TestListOrderBy(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	records := []sql.Record{
		{"name": "Charlie", "age": int64(35)},
		{"name": "Alice", "age": int64(30)},
		{"name": "Bob", "age": int64(25)},
	}
	if _, err := s.BatchInsert(ctx, "users", records); err != nil {
		t.Fatalf("BatchInsert: %v", err)
	}

	// Order by name ascending.
	var names []string
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		OrderBy: []sql.OrderBy{{Column: "name", Dir: sql.Asc}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	want := []string{"Alice", "Bob", "Charlie"}
	if !slices.Equal(names, want) {
		t.Fatalf("List order asc = %v, want %v", names, want)
	}

	// Order by age descending.
	names = nil
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		OrderBy: []sql.OrderBy{{Column: "age", Dir: sql.Desc}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	want = []string{"Charlie", "Alice", "Bob"}
	if !slices.Equal(names, want) {
		t.Fatalf("List order desc = %v, want %v", names, want)
	}
}

func TestListLimitOffset(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	for i := 0; i < 10; i++ {
		if _, err := s.Insert(ctx, "users", sql.Record{
			"name": string(rune('A' + i)),
			"age":  int64(20 + i),
		}); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	// Limit 3.
	var count int
	for _, err := range s.List(ctx, "users", &sql.ListOptions{Limit: 3}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		count++
	}
	if count != 3 {
		t.Fatalf("List limit=3: got %d records, want 3", count)
	}

	// Offset 8, should get 2 remaining.
	count = 0
	for _, err := range s.List(ctx, "users", &sql.ListOptions{Offset: 8}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		count++
	}
	if count != 2 {
		t.Fatalf("List offset=8: got %d records, want 2", count)
	}

	// Offset beyond range.
	count = 0
	for _, err := range s.List(ctx, "users", &sql.ListOptions{Offset: 100}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		count++
	}
	if count != 0 {
		t.Fatalf("List offset=100: got %d records, want 0", count)
	}
}

func TestListLike(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	records := []sql.Record{
		{"name": "Alice Anderson"},
		{"name": "Bob Brown"},
		{"name": "Alice Baker"},
	}
	if _, err := s.BatchInsert(ctx, "users", records); err != nil {
		t.Fatalf("BatchInsert: %v", err)
	}

	var names []string
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{
			{Column: "name", Op: sql.OpLike, Value: "alice%"},
		},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	want := []string{"Alice Anderson", "Alice Baker"}
	if !slices.Equal(names, want) {
		t.Fatalf("List LIKE = %v, want %v", names, want)
	}
}

func TestBatchInsertBatchDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	records := []sql.Record{
		{"name": "Alice", "age": int64(30)},
		{"name": "Bob", "age": int64(25)},
		{"name": "Charlie", "age": int64(35)},
	}
	if _, err := s.BatchInsert(ctx, "users", records); err != nil {
		t.Fatalf("BatchInsert: %v", err)
	}

	// Collect all IDs.
	var ids []int64
	for r, err := range s.List(ctx, "users", nil) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		ids = append(ids, r[sql.RowID].(int64))
	}
	if len(ids) != 3 {
		t.Fatalf("got %d records, want 3", len(ids))
	}

	// BatchDelete first two.
	if err := s.BatchDelete(ctx, "users", ids[:2]); err != nil {
		t.Fatalf("BatchDelete: %v", err)
	}

	// First two gone, third remains.
	_, err := s.Get(ctx, "users", ids[0])
	if !errors.Is(err, sql.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for id %d, got %v", ids[0], err)
	}
	_, err = s.Get(ctx, "users", ids[1])
	if !errors.Is(err, sql.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for id %d, got %v", ids[1], err)
	}
	r, err := s.Get(ctx, "users", ids[2])
	if err != nil {
		t.Fatalf("Get remaining: %v", err)
	}
	if r["name"] != "Charlie" {
		t.Fatalf("remaining record name = %v, want Charlie", r["name"])
	}
}

func TestAutoIncrementIDs(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	id1, _ := s.Insert(ctx, "users", sql.Record{"name": "A"})
	id2, _ := s.Insert(ctx, "users", sql.Record{"name": "B"})
	id3, _ := s.Insert(ctx, "users", sql.Record{"name": "C"})

	if id1 >= id2 || id2 >= id3 {
		t.Fatalf("IDs should be strictly increasing: %d, %d, %d", id1, id2, id3)
	}
}

func TestValueIsolation(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	original := sql.Record{"name": "Alice", "age": int64(30)}
	id, _ := s.Insert(ctx, "users", original)

	// Mutate original map — store should not be affected.
	original["name"] = "MUTATED"

	r, _ := s.Get(ctx, "users", id)
	if r["name"] != "Alice" {
		t.Fatal("store value was mutated via original map")
	}

	// Mutate returned map — store should not be affected.
	r["name"] = "MUTATED"
	r2, _ := s.Get(ctx, "users", id)
	if r2["name"] != "Alice" {
		t.Fatal("store value was mutated via returned map")
	}
}

func TestNilConditionValue(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	records := []sql.Record{
		{"name": "Alice", "age": int64(30)},
		{"name": "Bob"},
	}
	if _, err := s.BatchInsert(ctx, "users", records); err != nil {
		t.Fatalf("BatchInsert: %v", err)
	}

	// Filter where age != nil (OpNe with nil) — should return only Alice.
	var names []string
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{
			{Column: "age", Op: sql.OpNe, Value: nil},
		},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	if len(names) != 1 || names[0] != "Alice" {
		t.Fatalf("List OpNe nil = %v, want [Alice]", names)
	}
}

func TestUpdateIgnoresRowID(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	id, _ := s.Insert(ctx, "users", sql.Record{"name": "Alice"})

	// Attempt to change RowID via Update — should be ignored.
	err := s.Update(ctx, "users", id, sql.Record{sql.RowID: int64(999), "name": "Bob"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	r, _ := s.Get(ctx, "users", id)
	if r[sql.RowID] != id {
		t.Fatalf("RowID changed to %v after Update with _rowid field", r[sql.RowID])
	}
	if r["name"] != "Bob" {
		t.Fatalf("name = %v, want Bob", r["name"])
	}
}

func TestListEmptyTable(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	var count int
	for _, err := range s.List(ctx, "users", nil) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		count++
	}
	if count != 0 {
		t.Fatalf("List empty table: got %d records, want 0", count)
	}
}

func TestListFilterNe(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	records := []sql.Record{
		{"name": "Alice", "age": int64(30)},
		{"name": "Bob", "age": int64(25)},
		{"name": "Charlie", "age": int64(30)},
	}
	if _, err := s.BatchInsert(ctx, "users", records); err != nil {
		t.Fatalf("BatchInsert: %v", err)
	}

	var names []string
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{
			{Column: "age", Op: sql.OpNe, Value: int64(30)},
		},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	if len(names) != 1 || names[0] != "Bob" {
		t.Fatalf("List OpNe = %v, want [Bob]", names)
	}
}

func TestListFilterMixedNumericTypes(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	records := []sql.Record{
		{"name": "A", "score": 10.5},
		{"name": "B", "score": 20.0},
		{"name": "C", "score": 30.5},
	}
	if _, err := s.BatchInsert(ctx, "users", records); err != nil {
		t.Fatalf("BatchInsert: %v", err)
	}

	// float64 vs float64 comparison.
	var names []string
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{{Column: "score", Op: sql.OpGt, Value: 15.0}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	if !slices.Equal(names, []string{"B", "C"}) {
		t.Fatalf("float64 filter = %v, want [B, C]", names)
	}

	// int value vs int64 record value.
	s2 := newTestStore(t)
	createUsersTable(t, s2)
	if _, err := s2.BatchInsert(ctx, "users", []sql.Record{
		{"name": "X", "age": int64(10)},
		{"name": "Y", "age": int64(20)},
	}); err != nil {
		t.Fatalf("BatchInsert: %v", err)
	}
	names = nil
	for r, err := range s2.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{{Column: "age", Op: sql.OpEq, Value: int(20)}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	if !slices.Equal(names, []string{"Y"}) {
		t.Fatalf("int vs int64 = %v, want [Y]", names)
	}

	// int record value vs int filter value.
	s3 := newTestStore(t)
	createUsersTable(t, s3)
	if _, err := s3.BatchInsert(ctx, "users", []sql.Record{
		{"name": "P", "age": int(5)},
		{"name": "Q", "age": int(15)},
	}); err != nil {
		t.Fatalf("BatchInsert: %v", err)
	}
	names = nil
	for r, err := range s3.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{{Column: "age", Op: sql.OpGt, Value: int(10)}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	if !slices.Equal(names, []string{"Q"}) {
		t.Fatalf("int vs int = %v, want [Q]", names)
	}

	// int record vs int64 filter.
	names = nil
	for r, err := range s3.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{{Column: "age", Op: sql.OpLe, Value: int64(5)}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	if !slices.Equal(names, []string{"P"}) {
		t.Fatalf("int vs int64 Le = %v, want [P]", names)
	}

	// float64 record vs int64 filter.
	names = nil
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{{Column: "score", Op: sql.OpLt, Value: int64(15)}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	if !slices.Equal(names, []string{"A"}) {
		t.Fatalf("float64 vs int64 = %v, want [A]", names)
	}

	// Incomparable types should not match.
	names = nil
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{{Column: "score", Op: sql.OpEq, Value: "not a number"}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	if len(names) != 0 {
		t.Fatalf("incomparable types = %v, want empty", names)
	}
}

func TestListLikeWildcards(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	records := []sql.Record{
		{"name": "cat"},
		{"name": "car"},
		{"name": "bat"},
		{"name": "cut"},
	}
	if _, err := s.BatchInsert(ctx, "users", records); err != nil {
		t.Fatalf("BatchInsert: %v", err)
	}

	// _ wildcard: "c_t" matches "cat" and "cut".
	var names []string
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{{Column: "name", Op: sql.OpLike, Value: "c_t"}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	want := []string{"cat", "cut"}
	if !slices.Equal(names, want) {
		t.Fatalf("LIKE c_t = %v, want %v", names, want)
	}

	// Exact match without wildcards.
	names = nil
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{{Column: "name", Op: sql.OpLike, Value: "bat"}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	if !slices.Equal(names, []string{"bat"}) {
		t.Fatalf("LIKE exact = %v, want [bat]", names)
	}

	// Middle % wildcard: "c%t" matches "cat" and "cut".
	names = nil
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{{Column: "name", Op: sql.OpLike, Value: "c%t"}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	if !slices.Equal(names, want) {
		t.Fatalf("LIKE c%%t = %v, want %v", names, want)
	}

	// No match.
	names = nil
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{{Column: "name", Op: sql.OpLike, Value: "xyz%"}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	if len(names) != 0 {
		t.Fatalf("LIKE xyz%% = %v, want empty", names)
	}
}

func TestListEqNilBothSides(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	records := []sql.Record{
		{"name": "Alice", "age": int64(30)},
		{"name": "Bob"},
	}
	if _, err := s.BatchInsert(ctx, "users", records); err != nil {
		t.Fatalf("BatchInsert: %v", err)
	}

	// Filter where age == nil (missing field matches nil).
	var names []string
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{{Column: "age", Op: sql.OpEq, Value: nil}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	if !slices.Equal(names, []string{"Bob"}) {
		t.Fatalf("OpEq nil = %v, want [Bob]", names)
	}

	// Filter where missing_col < 5 (missing column, non-nil value).
	names = nil
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{{Column: "missing_col", Op: sql.OpLt, Value: int64(5)}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	if len(names) != 0 {
		t.Fatalf("OpLt on missing col = %v, want empty", names)
	}
}

func TestListFilterLtLe(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	records := []sql.Record{
		{"name": "A", "age": int64(10)},
		{"name": "B", "age": int64(20)},
		{"name": "C", "age": int64(30)},
	}
	if _, err := s.BatchInsert(ctx, "users", records); err != nil {
		t.Fatalf("BatchInsert: %v", err)
	}

	// OpLt 20 → only A.
	var names []string
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{{Column: "age", Op: sql.OpLt, Value: int64(20)}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	if !slices.Equal(names, []string{"A"}) {
		t.Fatalf("OpLt = %v, want [A]", names)
	}

	// OpLe 20 → A, B.
	names = nil
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{{Column: "age", Op: sql.OpLe, Value: int64(20)}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	if !slices.Equal(names, []string{"A", "B"}) {
		t.Fatalf("OpLe = %v, want [A, B]", names)
	}
}

func TestSchemaValidation(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	err := s.CreateTable(ctx, "strict", []sql.Column{
		{Name: "name", Type: sql.TypeText},
		{Name: "value", Type: sql.TypeInteger},
	})
	if err != nil {
		t.Fatalf("CreateTable: %v", err)
	}

	// Unknown column on Insert.
	_, err = s.Insert(ctx, "strict", sql.Record{"name": "a", "value": int64(1), "bogus": "x"})
	if !errors.Is(err, sql.ErrUnknownColumn) {
		t.Fatalf("Insert unknown col: expected ErrUnknownColumn, got %v", err)
	}

	// Null non-nullable column on Insert (missing required "value").
	_, err = s.Insert(ctx, "strict", sql.Record{"name": "a"})
	if !errors.Is(err, sql.ErrNullValue) {
		t.Fatalf("Insert null non-nullable: expected ErrNullValue, got %v", err)
	}

	// Valid insert.
	id, err := s.Insert(ctx, "strict", sql.Record{"name": "a", "value": int64(1)})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// Unknown column on Update.
	err = s.Update(ctx, "strict", id, sql.Record{"bogus": "x"})
	if !errors.Is(err, sql.ErrUnknownColumn) {
		t.Fatalf("Update unknown col: expected ErrUnknownColumn, got %v", err)
	}

	// Null non-nullable on Update.
	err = s.Update(ctx, "strict", id, sql.Record{"value": nil})
	if !errors.Is(err, sql.ErrNullValue) {
		t.Fatalf("Update null non-nullable: expected ErrNullValue, got %v", err)
	}

	// Unknown column on BatchInsert.
	_, err = s.BatchInsert(ctx, "strict", []sql.Record{{"name": "b", "value": int64(2), "bogus": "y"}})
	if !errors.Is(err, sql.ErrUnknownColumn) {
		t.Fatalf("BatchInsert unknown col: expected ErrUnknownColumn, got %v", err)
	}

	// BatchInsert returns IDs.
	ids, err := s.BatchInsert(ctx, "strict", []sql.Record{
		{"name": "c", "value": int64(3)},
		{"name": "d", "value": int64(4)},
	})
	if err != nil {
		t.Fatalf("BatchInsert: %v", err)
	}
	if len(ids) != 2 || ids[0] >= ids[1] {
		t.Fatalf("BatchInsert ids = %v, want 2 strictly increasing ids", ids)
	}
}

func TestListLikeUTF8(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	records := []sql.Record{
		{"name": "你好世界"},
		{"name": "你好中国"},
		{"name": "hello"},
	}
	if _, err := s.BatchInsert(ctx, "users", records); err != nil {
		t.Fatalf("BatchInsert: %v", err)
	}

	// _ should match one character (rune), not one byte.
	var names []string
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{{Column: "name", Op: sql.OpLike, Value: "你好__"}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	want := []string{"你好世界", "你好中国"}
	if !slices.Equal(names, want) {
		t.Fatalf("LIKE UTF-8 = %v, want %v", names, want)
	}
}

func TestListOrderByWithNil(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	records := []sql.Record{
		{"name": "Alice", "age": int64(30)},
		{"name": "Bob"},
		{"name": "Charlie", "age": int64(20)},
	}
	if _, err := s.BatchInsert(ctx, "users", records); err != nil {
		t.Fatalf("BatchInsert: %v", err)
	}

	// Order by age ascending — nil should be last (NULLS LAST).
	var names []string
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		OrderBy: []sql.OrderBy{{Column: "age", Dir: sql.Asc}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	want := []string{"Charlie", "Alice", "Bob"}
	if !slices.Equal(names, want) {
		t.Fatalf("OrderBy with nil asc = %v, want %v", names, want)
	}

	// Order by age descending — nil should still be last.
	names = nil
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		OrderBy: []sql.OrderBy{{Column: "age", Dir: sql.Desc}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	want = []string{"Alice", "Charlie", "Bob"}
	if !slices.Equal(names, want) {
		t.Fatalf("OrderBy with nil desc = %v, want %v", names, want)
	}
}

func TestCompareIntFloat64(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	records := []sql.Record{
		{"name": "A", "age": int(10)},
		{"name": "B", "age": int(20)},
	}
	if _, err := s.BatchInsert(ctx, "users", records); err != nil {
		t.Fatalf("BatchInsert: %v", err)
	}

	// int record vs float64 filter.
	var names []string
	for r, err := range s.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{{Column: "age", Op: sql.OpGt, Value: float64(15)}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	if !slices.Equal(names, []string{"B"}) {
		t.Fatalf("int vs float64 = %v, want [B]", names)
	}

	// float64 record vs int filter.
	s2 := newTestStore(t)
	createUsersTable(t, s2)
	if _, err := s2.BatchInsert(ctx, "users", []sql.Record{
		{"name": "X", "score": 10.5},
		{"name": "Y", "score": 25.0},
	}); err != nil {
		t.Fatalf("BatchInsert: %v", err)
	}
	names = nil
	for r, err := range s2.List(ctx, "users", &sql.ListOptions{
		Where: []sql.Condition{{Column: "score", Op: sql.OpGe, Value: int(20)}},
	}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names = append(names, r["name"].(string))
	}
	if !slices.Equal(names, []string{"Y"}) {
		t.Fatalf("float64 vs int = %v, want [Y]", names)
	}
}

func TestBlobIsolation(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	err := s.CreateTable(ctx, "blobs", []sql.Column{
		{Name: "name", Type: sql.TypeText},
		{Name: "data", Type: sql.TypeBlob, Nullable: true},
	})
	if err != nil {
		t.Fatalf("CreateTable: %v", err)
	}

	original := []byte("hello")
	id, err := s.Insert(ctx, "blobs", sql.Record{"name": "a", "data": original})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// Mutate the original slice — store should not be affected.
	original[0] = 'X'
	r, _ := s.Get(ctx, "blobs", id)
	if got := r["data"].([]byte); string(got) != "hello" {
		t.Fatalf("store blob mutated via original: got %q", got)
	}

	// Mutate the returned slice — store should not be affected.
	returned, _ := s.Get(ctx, "blobs", id)
	returned["data"].([]byte)[0] = 'Y'
	r2, _ := s.Get(ctx, "blobs", id)
	if got := r2["data"].([]byte); string(got) != "hello" {
		t.Fatalf("store blob mutated via returned: got %q", got)
	}
}

func TestWriteNormalization(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	createUsersTable(t, s)

	// Insert with int and float32 — should be normalized to int64 and float64.
	id, err := s.Insert(ctx, "users", sql.Record{
		"name": "Alice", "age": int(30), "score": float32(95.5),
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	r, _ := s.Get(ctx, "users", id)
	if _, ok := r["age"].(int64); !ok {
		t.Fatalf("age type = %T, want int64", r["age"])
	}
	if _, ok := r["score"].(float64); !ok {
		t.Fatalf("score type = %T, want float64", r["score"])
	}
	if r["age"].(int64) != 30 {
		t.Fatalf("age = %v, want 30", r["age"])
	}

	// Update with int and float32 — also normalized.
	err = s.Update(ctx, "users", id, sql.Record{"age": int(31), "score": float32(96.0)})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	r, _ = s.Get(ctx, "users", id)
	if _, ok := r["age"].(int64); !ok {
		t.Fatalf("age type after Update = %T, want int64", r["age"])
	}
	if _, ok := r["score"].(float64); !ok {
		t.Fatalf("score type after Update = %T, want float64", r["score"])
	}

	// BatchInsert with int and float32.
	ids, err := s.BatchInsert(ctx, "users", []sql.Record{
		{"name": "Bob", "age": int(25), "score": float32(88.0)},
	})
	if err != nil {
		t.Fatalf("BatchInsert: %v", err)
	}

	r, _ = s.Get(ctx, "users", ids[0])
	if _, ok := r["age"].(int64); !ok {
		t.Fatalf("BatchInsert age type = %T, want int64", r["age"])
	}
	if _, ok := r["score"].(float64); !ok {
		t.Fatalf("BatchInsert score type = %T, want float64", r["score"])
	}
}

func TestUpdateBlobIsolation(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	err := s.CreateTable(ctx, "blobs", []sql.Column{
		{Name: "name", Type: sql.TypeText},
		{Name: "data", Type: sql.TypeBlob, Nullable: true},
	})
	if err != nil {
		t.Fatalf("CreateTable: %v", err)
	}

	id, err := s.Insert(ctx, "blobs", sql.Record{"name": "a", "data": []byte("original")})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// Update with a blob and then mutate the source — store should not be affected.
	updateData := []byte("updated")
	if err := s.Update(ctx, "blobs", id, sql.Record{"data": updateData}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updateData[0] = 'X'

	r, _ := s.Get(ctx, "blobs", id)
	if got := r["data"].([]byte); string(got) != "updated" {
		t.Fatalf("store blob mutated via Update source: got %q", got)
	}
}

func TestTypeMismatch(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	err := s.CreateTable(ctx, "typed", []sql.Column{
		{Name: "name", Type: sql.TypeText},
		{Name: "count", Type: sql.TypeInteger},
		{Name: "rate", Type: sql.TypeReal, Nullable: true},
		{Name: "data", Type: sql.TypeBlob, Nullable: true},
	})
	if err != nil {
		t.Fatalf("CreateTable: %v", err)
	}

	// Insert: wrong type for text column.
	_, err = s.Insert(ctx, "typed", sql.Record{"name": 123, "count": int64(1)})
	if !errors.Is(err, sql.ErrTypeMismatch) {
		t.Fatalf("Insert int into text: expected ErrTypeMismatch, got %v", err)
	}

	// Insert: wrong type for integer column.
	_, err = s.Insert(ctx, "typed", sql.Record{"name": "a", "count": "not_int"})
	if !errors.Is(err, sql.ErrTypeMismatch) {
		t.Fatalf("Insert string into integer: expected ErrTypeMismatch, got %v", err)
	}

	// Insert: wrong type for real column.
	_, err = s.Insert(ctx, "typed", sql.Record{"name": "a", "count": int64(1), "rate": "bad"})
	if !errors.Is(err, sql.ErrTypeMismatch) {
		t.Fatalf("Insert string into real: expected ErrTypeMismatch, got %v", err)
	}

	// Insert: wrong type for blob column.
	_, err = s.Insert(ctx, "typed", sql.Record{"name": "a", "count": int64(1), "data": 42})
	if !errors.Is(err, sql.ErrTypeMismatch) {
		t.Fatalf("Insert int into blob: expected ErrTypeMismatch, got %v", err)
	}

	// Insert: correct types should succeed.
	id, err := s.Insert(ctx, "typed", sql.Record{
		"name": "ok", "count": int64(1), "rate": 3.14, "data": []byte("bin"),
	})
	if err != nil {
		t.Fatalf("Insert valid: %v", err)
	}

	// Insert: int (not int64) for integer column should also succeed.
	_, err = s.Insert(ctx, "typed", sql.Record{"name": "ok2", "count": int(2)})
	if err != nil {
		t.Fatalf("Insert int for integer column: %v", err)
	}

	// Update: wrong type.
	err = s.Update(ctx, "typed", id, sql.Record{"count": "oops"})
	if !errors.Is(err, sql.ErrTypeMismatch) {
		t.Fatalf("Update wrong type: expected ErrTypeMismatch, got %v", err)
	}

	// Update: nil for nullable is ok.
	err = s.Update(ctx, "typed", id, sql.Record{"rate": nil})
	if err != nil {
		t.Fatalf("Update nullable to nil: %v", err)
	}

	// BatchInsert: wrong type in batch.
	_, err = s.BatchInsert(ctx, "typed", []sql.Record{
		{"name": "good", "count": int64(1)},
		{"name": 999, "count": int64(2)},
	})
	if !errors.Is(err, sql.ErrTypeMismatch) {
		t.Fatalf("BatchInsert wrong type: expected ErrTypeMismatch, got %v", err)
	}
}
