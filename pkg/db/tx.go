package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
	"github.com/stashapp/stash/pkg/logger"
)

const (
	slowLogTime = time.Millisecond * 200
)

type Queryer interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)

	Rebind(query string) string
}

type stmt struct {
	stmt  *sql.Stmt
	query string
}

func logSQL(start time.Time, query string, args ...interface{}) {
	since := time.Since(start)
	if since >= slowLogTime {
		logger.Debugf("SLOW SQL [%v]: %s, args: %v", since, query, args)
	} else {
		logger.Tracef("SQL [%v]: %s, args: %v", since, query, args)
	}
}

type dbWrapper struct {
	tx      txWrapper
	driver  string
	dialect goqu.SQLDialect
}

func (w *dbWrapper) Insert(ctx context.Context, query *goqu.InsertDataset) (sql.Result, error) {
	q, args, err := query.SetDialect(w.dialect).ToSQL()
	if err != nil {
		return nil, err
	}

	return w.tx.Exec(ctx, q, args...)
}

func (w *dbWrapper) Update(ctx context.Context, query *goqu.UpdateDataset) (sql.Result, error) {
	q, args, err := query.SetDialect(w.dialect).ToSQL()
	if err != nil {
		return nil, err
	}

	return w.tx.Exec(ctx, q, args...)
}

func (w *dbWrapper) Destroy(ctx context.Context, query *goqu.DeleteDataset) (sql.Result, error) {
	q, args, err := query.SetDialect(w.dialect).ToSQL()
	if err != nil {
		return nil, err
	}

	return w.tx.Exec(ctx, q, args...)
}

func (w *dbWrapper) Get(ctx context.Context, dest interface{}, query *goqu.SelectDataset) error {
	q, args, err := query.SetDialect(w.dialect).ToSQL()
	if err != nil {
		return err
	}

	return w.tx.Get(ctx, dest, q, args...)
}

func (w *dbWrapper) Select(ctx context.Context, dest interface{}, query *goqu.SelectDataset) error {
	q, args, err := query.SetDialect(w.dialect).ToSQL()
	if err != nil {
		return err
	}

	return w.tx.Select(ctx, dest, q, args...)
}

func (w *dbWrapper) Query(ctx context.Context, query *goqu.SelectDataset) (*sqlx.Rows, error) {
	q, args, err := query.SetDialect(w.dialect).ToSQL()
	if err != nil {
		return nil, err
	}

	return w.tx.Query(ctx, q, args...)
}

func (w *dbWrapper) InsertID(ctx context.Context, query *goqu.InsertDataset) (int, error) {
	q, args, err := query.Returning(idColumn).SetDialect(w.dialect).ToSQL()
	if err != nil {
		return 0, err
	}

	return w.tx.GetInt(ctx, q, args...)
}

func (w *dbWrapper) GetInt(ctx context.Context, query *goqu.SelectDataset) (int, error) {
	q, args, err := query.SetDialect(w.dialect).ToSQL()
	if err != nil {
		return 0, err
	}

	return w.tx.GetInt(ctx, q, args...)
}

func (w *dbWrapper) GetInts(ctx context.Context, query *goqu.SelectDataset) ([]int, error) {
	q, args, err := query.SetDialect(w.dialect).ToSQL()
	if err != nil {
		return nil, err
	}

	return w.tx.GetInts(ctx, q, args...)
}

func (w *dbWrapper) QueryFunc(ctx context.Context, query *goqu.SelectDataset, single bool, f func(rows *sqlx.Rows) error) error {
	q, args, err := query.SetDialect(w.dialect).ToSQL()
	if err != nil {
		return err
	}

	return w.tx.QueryFunc(ctx, q, args, single, f)
}

type txWrapper struct {
	tx Queryer
}

func sqlError(err error, sql string, args ...interface{}) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("error executing `%s` [%v]: %w", sql, args, err)
}

func (w *txWrapper) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if len(args) > 0 {
		// query = sanitizeQuery(query)
		query = w.tx.Rebind(query)
	}

	start := time.Now()
	ret, err := w.tx.ExecContext(ctx, query, args...)
	logSQL(start, query, args...)

	return ret, sqlError(err, query, args...)
}

func (w *txWrapper) Get(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	if len(args) > 0 {
		// query = sanitizeQuery(query)
		query = w.tx.Rebind(query)
	}

	start := time.Now()
	err := w.tx.GetContext(ctx, dest, query, args...)
	logSQL(start, query, args...)

	return sqlError(err, query, args...)
}

func (w *txWrapper) Select(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	if len(args) > 0 {
		// query = sanitizeQuery(query)
		query = w.tx.Rebind(query)
	}

	start := time.Now()
	err := w.tx.SelectContext(ctx, dest, query, args...)
	logSQL(start, query, args...)

	return sqlError(err, query, args...)
}

func (w *txWrapper) Query(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	if len(args) > 0 {
		// query = sanitizeQuery(query)
		query = w.tx.Rebind(query)
	}

	start := time.Now()
	ret, err := w.tx.QueryxContext(ctx, query, args...)
	logSQL(start, query, args...)

	return ret, sqlError(err, query, args...)
}

// Prepare creates a prepared statement.
func (w *txWrapper) Prepare(ctx context.Context, query string) (*stmt, error) {
	// query = sanitizeQuery(query)
	query = w.tx.Rebind(query)

	// nolint:sqlclosecheck
	ret, err := w.tx.PrepareContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error preparing `%s`: %w", query, err)
	}

	return &stmt{
		query: query,
		stmt:  ret,
	}, nil
}

func (w *txWrapper) GetInt(ctx context.Context, query string, args ...interface{}) (int, error) {
	if len(args) > 0 {
		// query = sanitizeQuery(query)
		query = w.tx.Rebind(query)
	}

	start := time.Now()

	var ret int
	err := w.tx.GetContext(ctx, &ret, query, args...)
	logSQL(start, query, args...)

	return ret, sqlError(err, query, args...)
}

func (w *txWrapper) GetInts(ctx context.Context, query string, args ...interface{}) ([]int, error) {
	if len(args) > 0 {
		// query = sanitizeQuery(query)
		query = w.tx.Rebind(query)
	}

	start := time.Now()

	var ret []int
	err := w.tx.SelectContext(ctx, &ret, query, args...)
	logSQL(start, query, args...)

	return ret, sqlError(err, query, args...)
}

func (w *txWrapper) QueryFunc(ctx context.Context, query string, args []interface{}, single bool, f func(rows *sqlx.Rows) error) error {
	if len(args) > 0 {
		// query = sanitizeQuery(query)
		query = w.tx.Rebind(query)
	}

	rows, err := w.Query(ctx, query, args...)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		if err := f(rows); err != nil {
			return err
		}
		if single {
			break
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	return nil
}

func sanitizeQuery(query string) string {
	var builder strings.Builder
	builder.Grow(len(query))

	i := 1
	for _, r := range query {
		if r != '?' {
			builder.WriteRune(r)
			continue
		}

		builder.WriteRune('$')
		builder.WriteString(strconv.Itoa(i))
		i++
	}

	return builder.String()
}

func (s *stmt) Close() error {
	return s.stmt.Close()
}

// Exec executes a prepared statement.
func (s *stmt) Exec(ctx context.Context, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	ret, err := s.stmt.ExecContext(ctx, args...)
	logSQL(start, s.query, args...)

	return ret, sqlError(err, s.query, args...)
}

func (s *stmt) Query(ctx context.Context, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	ret, err := s.stmt.QueryContext(ctx, args...)
	logSQL(start, s.query, args...)

	return ret, sqlError(err, s.query, args...)
}

func insert(ctx context.Context, query *goqu.InsertDataset) (sql.Result, error) {
	db, err := getDBWrapper(ctx)
	if err != nil {
		return nil, err
	}

	return db.Insert(ctx, query)
}

func insertID(ctx context.Context, query *goqu.InsertDataset) (int, error) {
	db, err := getDBWrapper(ctx)
	if err != nil {
		return 0, err
	}

	return db.InsertID(ctx, query)
}

func update(ctx context.Context, query *goqu.UpdateDataset) (sql.Result, error) {
	db, err := getDBWrapper(ctx)
	if err != nil {
		return nil, err
	}

	return db.Update(ctx, query)
}

func destroy(ctx context.Context, query *goqu.DeleteDataset) (sql.Result, error) {
	db, err := getDBWrapper(ctx)
	if err != nil {
		return nil, err
	}

	return db.Destroy(ctx, query)
}

// func query(ctx context.Context, query *goqu.SelectDataset) (*sqlx.Rows, error) {
// 	db, err := getDBWrapper(ctx)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	return db.Query(ctx, query)
// }

func queryValue(ctx context.Context, dest interface{}, query *goqu.SelectDataset) error {
	db, err := getDBWrapper(ctx)
	if err != nil {
		return err
	}

	return db.Get(ctx, dest, query)
}

func queryInt(ctx context.Context, query *goqu.SelectDataset) (int, error) {
	db, err := getDBWrapper(ctx)
	if err != nil {
		return 0, err
	}

	return db.GetInt(ctx, query)
}

func queryInts(ctx context.Context, query *goqu.SelectDataset) ([]int, error) {
	db, err := getDBWrapper(ctx)
	if err != nil {
		return nil, err
	}

	return db.GetInts(ctx, query)
}

func querySelect(ctx context.Context, dest interface{}, query *goqu.SelectDataset) error {
	db, err := getDBWrapper(ctx)
	if err != nil {
		return err
	}

	return db.Select(ctx, dest, query)
}

func queryFunc(ctx context.Context, query *goqu.SelectDataset, single bool, f func(rows *sqlx.Rows) error) error {
	db, err := getDBWrapper(ctx)
	if err != nil {
		return err
	}

	return db.QueryFunc(ctx, query, single, f)
}
