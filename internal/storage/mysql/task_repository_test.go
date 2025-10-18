package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMemoryTaskRepositoryCRUD(t *testing.T) {
	t.Parallel()

	repo, err := NewMemoryTaskRepository(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create memory repo: %v", err)
	}

	ctx := context.Background()
	now := time.Now().Unix()
	record := &TaskRecord{
		Goal:      "goal",
		Thought:   "thought",
		Reply:     "reply",
		Observes:  "observe",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := repo.Create(ctx, record); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if record.ID == 0 {
		t.Fatalf("expected record ID to be assigned")
	}

	stored, err := repo.GetByID(ctx, record.ID)
	if err != nil {
		t.Fatalf("get by id failed: %v", err)
	}
	if stored.Reply != "reply" {
		t.Fatalf("unexpected reply: %s", stored.Reply)
	}

	record.Reply = "updated"
	record.UpdatedAt = now + 10
	if err := repo.Update(ctx, *record); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	list, err := repo.ListLatest(ctx, 10)
	if err != nil {
		t.Fatalf("list latest failed: %v", err)
	}
	if len(list) != 1 || list[0].Reply != "updated" {
		t.Fatalf("unexpected list result: %+v", list)
	}

	if err := repo.Delete(ctx, record.ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if _, err := repo.GetByID(ctx, record.ID); err == nil {
		t.Fatalf("expected error after delete")
	}

	err = repo.WithTransaction(ctx, func(ctx context.Context, tx TaskRepository) error {
		r1 := &TaskRecord{Goal: "tx-1", CreatedAt: now + 20, UpdatedAt: now + 20}
		if err := tx.Create(ctx, r1); err != nil {
			return err
		}
		r2 := &TaskRecord{Goal: "tx-2", CreatedAt: now + 30, UpdatedAt: now + 30}
		if err := tx.Create(ctx, r2); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}

	txList, err := repo.ListLatest(ctx, 10)
	if err != nil {
		t.Fatalf("list after tx failed: %v", err)
	}
	if len(txList) != 2 {
		t.Fatalf("expected 2 records, got %d", len(txList))
	}
	if txList[0].Goal != "tx-2" {
		t.Fatalf("records not sorted by created_at desc: %+v", txList)
	}
}

func TestSQLTaskRepositoryCreate(t *testing.T) {
	t.Parallel()

	db, driver := newMockDB(t, []mockOperation{
		execOp(insertTaskSQL(), mockResult{lastInsertID: 42, rowsAffected: 1}),
	})
	defer driver.assertConsumed(t)
	defer db.Close()

	repo := &SQLTaskRepository{db: db}
	record := &TaskRecord{Goal: "goal", Thought: "thought", Reply: "reply", CreatedAt: 1, UpdatedAt: 1}
	if err := repo.Create(context.Background(), record); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if record.ID != 42 {
		t.Fatalf("expected id 42, got %d", record.ID)
	}
}

func TestSQLTaskRepositoryGetUpdateDelete(t *testing.T) {
	t.Parallel()

	rows := mockRowsData{
		columns: []string{"id", "goal", "chain_action", "address", "thought", "reply", "chain_id", "block_number", "observes", "created_at", "updated_at"},
		values:  [][]driver.Value{{int64(7), "goal", "", "", "thought", "reply", "", "", "ob", int64(1), int64(1)}},
	}

	db, driver := newMockDB(t, []mockOperation{
		queryOp(`SELECT id, goal, chain_action, address, thought, reply, chain_id, block_number, observes, created_at, updated_at
    FROM tasks WHERE id = ?`, rows),
		execOp(updateTaskSQL(), mockResult{rowsAffected: 1}),
		execOp(`DELETE FROM tasks WHERE id = ?`, mockResult{rowsAffected: 1}),
	})
	defer driver.assertConsumed(t)
	defer db.Close()

	repo := &SQLTaskRepository{db: db}
	rec, err := repo.GetByID(context.Background(), 7)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if rec.ID != 7 || rec.Thought != "thought" {
		t.Fatalf("unexpected record: %+v", rec)
	}

	rec.Reply = "new"
	if err := repo.Update(context.Background(), *rec); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if err := repo.Delete(context.Background(), 7); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
}

func TestSQLTaskRepositoryListLatest(t *testing.T) {
	t.Parallel()

	rows := mockRowsData{
		columns: []string{"id", "goal", "chain_action", "address", "thought", "reply", "chain_id", "block_number", "observes", "created_at", "updated_at"},
		values: [][]driver.Value{
			{int64(2), "g2", "", "", "t2", "r2", "", "", "o2", int64(20), int64(20)},
			{int64(1), "g1", "", "", "t1", "r1", "", "", "o1", int64(10), int64(10)},
		},
	}

	db, driver := newMockDB(t, []mockOperation{
		queryOp(`SELECT id, goal, chain_action, address, thought, reply, chain_id, block_number, observes, created_at, updated_at
    FROM tasks ORDER BY created_at DESC, id DESC LIMIT ?`, rows),
	})
	defer driver.assertConsumed(t)
	defer db.Close()

	repo := &SQLTaskRepository{db: db}
	list, err := repo.ListLatest(context.Background(), 2)
	if err != nil {
		t.Fatalf("list latest failed: %v", err)
	}
	if len(list) != 2 || list[0].ID != 2 {
		t.Fatalf("unexpected list: %+v", list)
	}
}

func TestSQLTaskRepositoryWithTransaction(t *testing.T) {
	t.Parallel()

	ops := []mockOperation{
		beginOp(),
		execOp(insertTaskSQL(), mockResult{lastInsertID: 1, rowsAffected: 1}),
		commitOp(),
	}
	db, driver := newMockDB(t, ops)
	defer driver.assertConsumed(t)
	defer db.Close()

	repo := &SQLTaskRepository{db: db}
	err := repo.WithTransaction(context.Background(), func(ctx context.Context, tx TaskRepository) error {
		rec := &TaskRecord{Goal: "goal", Thought: "t", Reply: "r", CreatedAt: 1, UpdatedAt: 1}
		return tx.Create(ctx, rec)
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}
}

func TestSQLTaskRepositoryRunMigrations(t *testing.T) {
	t.Parallel()

	ops := []mockOperation{
		execOp(`CREATE TABLE IF NOT EXISTS schema_migrations (
        version VARCHAR(32) NOT NULL PRIMARY KEY,
        applied_at BIGINT NOT NULL
)`, mockResult{}),
		queryOp(`SELECT version FROM schema_migrations`, mockRowsData{columns: []string{"version"}}),
		beginOp(),
		execOp(readMigrationStatement(), mockResult{rowsAffected: 0}),
		execOp(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, mockResult{rowsAffected: 1}),
		commitOp(),
	}
	db, driver := newMockDB(t, ops)
	defer driver.assertConsumed(t)
	defer db.Close()

	repo := &SQLTaskRepository{db: db}
	if err := repo.runMigrations(context.Background()); err != nil {
		t.Fatalf("run migrations failed: %v", err)
	}
}

func insertTaskSQL() string {
	return `INSERT INTO tasks
    (goal, chain_action, address, thought, reply, chain_id, block_number, observes, created_at, updated_at)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
}

func updateTaskSQL() string {
	return `UPDATE tasks SET goal = ?, chain_action = ?, address = ?, thought = ?, reply = ?, chain_id = ?, block_number = ?, observes = ?, created_at = ?, updated_at = ?
    WHERE id = ?`
}

func readMigrationStatement() string {
	content, err := embeddedMigrations.ReadFile("0001_create_tasks.sql")
	if err != nil {
		panic(fmt.Sprintf("failed to read migration: %v", err))
	}
	statements := splitSQLStatements(string(content))
	if len(statements) == 0 {
		panic("no statements in migration")
	}
	return statements[0]
}

type operationType int

const (
	opExec operationType = iota
	opQuery
	opBegin
	opCommit
	opRollback
)

type mockOperation struct {
	typ    operationType
	query  string
	result mockResult
	rows   mockRowsData
	err    error
}

type mockResult struct {
	lastInsertID int64
	rowsAffected int64
}

func (r mockResult) LastInsertId() (int64, error) { return r.lastInsertID, nil }
func (r mockResult) RowsAffected() (int64, error) { return r.rowsAffected, nil }

type mockRowsData struct {
	columns []string
	values  [][]driver.Value
}

type queueDriver struct {
	ops []mockOperation
	idx int32
}

var driverSeq atomic.Int32

func newMockDB(t *testing.T, ops []mockOperation) (*sql.DB, *queueDriver) {
	t.Helper()

	drv := &queueDriver{ops: ops}
	name := fmt.Sprintf("mock-mysql-%d", driverSeq.Add(1))
	sql.Register(name, drv)

	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("open mock db failed: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return db, drv
}

func execOp(query string, result mockResult) mockOperation {
	return mockOperation{typ: opExec, query: query, result: result}
}

func queryOp(query string, rows mockRowsData) mockOperation {
	return mockOperation{typ: opQuery, query: query, rows: rows}
}

func beginOp() mockOperation { return mockOperation{typ: opBegin} }

func commitOp() mockOperation { return mockOperation{typ: opCommit} }

func rollbackOp() mockOperation { return mockOperation{typ: opRollback} }

func (d *queueDriver) assertConsumed(t *testing.T) {
	t.Helper()

	if int(atomic.LoadInt32(&d.idx)) != len(d.ops) {
		t.Fatalf("not all operations consumed: %d/%d", atomic.LoadInt32(&d.idx), len(d.ops))
	}
}

func (d *queueDriver) Open(name string) (driver.Conn, error) {
	return &mockConn{driver: d}, nil
}

type mockConn struct {
	driver *queueDriver
}

func (c *mockConn) Prepare(query string) (driver.Stmt, error) {
	return nil, fmt.Errorf("prepare not supported: %s", query)
}

func (c *mockConn) Close() error { return nil }

func (c *mockConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *mockConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	op, err := c.next(opBegin, "")
	if err != nil {
		return nil, err
	}
	if op.err != nil {
		return nil, op.err
	}
	return &mockTx{driver: c.driver}, nil
}

func (c *mockConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	return c.ExecContext(context.Background(), query, named(args))
}

func (c *mockConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	op, err := c.next(opExec, query)
	if err != nil {
		return nil, err
	}
	if op.err != nil {
		return nil, op.err
	}
	return op.result, nil
}

func (c *mockConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	return c.QueryContext(context.Background(), query, named(args))
}

func (c *mockConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	op, err := c.next(opQuery, query)
	if err != nil {
		return nil, err
	}
	if op.err != nil {
		return nil, op.err
	}
	return &mockRows{columns: op.rows.columns, values: op.rows.values}, nil
}

func (c *mockConn) Ping(ctx context.Context) error { return nil }

func (c *mockConn) next(expected operationType, query string) (*mockOperation, error) {
	idx := int(atomic.LoadInt32(&c.driver.idx))
	if idx >= len(c.driver.ops) {
		return nil, fmt.Errorf("unexpected operation: %v", expected)
	}
	op := &c.driver.ops[idx]
	if op.typ != expected {
		return nil, fmt.Errorf("expected operation %v, got %v", expected, op.typ)
	}
	atomic.AddInt32(&c.driver.idx, 1)
	if op.query != "" {
		expectedSQL := normalizeSQL(op.query)
		actualSQL := normalizeSQL(query)
		if expectedSQL != actualSQL {
			return nil, fmt.Errorf("unexpected query. want %q got %q", expectedSQL, actualSQL)
		}
	}
	return op, nil
}

type mockTx struct {
	driver *queueDriver
}

func (t *mockTx) Commit() error {
	op, err := t.next(opCommit)
	if err != nil {
		return err
	}
	return op.err
}

func (t *mockTx) Rollback() error {
	op, err := t.next(opRollback)
	if err != nil {
		return err
	}
	return op.err
}

func (t *mockTx) next(expected operationType) (*mockOperation, error) {
	idx := int(atomic.LoadInt32(&t.driver.idx))
	if idx >= len(t.driver.ops) {
		return nil, fmt.Errorf("unexpected operation: %v", expected)
	}
	op := &t.driver.ops[idx]
	if op.typ != expected {
		return nil, fmt.Errorf("expected operation %v, got %v", expected, op.typ)
	}
	atomic.AddInt32(&t.driver.idx, 1)
	return op, nil
}

type mockRows struct {
	columns []string
	values  [][]driver.Value
	idx     int
}

func (r *mockRows) Columns() []string { return r.columns }
func (r *mockRows) Close() error      { return nil }

func (r *mockRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.idx])
	r.idx++
	return nil
}

func named(args []driver.Value) []driver.NamedValue {
	namedArgs := make([]driver.NamedValue, len(args))
	for i, arg := range args {
		namedArgs[i] = driver.NamedValue{Ordinal: i + 1, Value: arg}
	}
	return namedArgs
}

func normalizeSQL(query string) string {
	fields := strings.Fields(query)
	return strings.Join(fields, " ")
}
