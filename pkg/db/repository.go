package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jmoiron/sqlx"

	"github.com/stashapp/stash/pkg/models"
)

const idColumn = "id"

type repository struct {
	tableName string
	idColumn  string
}

func (r *repository) getAll(ctx context.Context, id int, f func(rows *sqlx.Rows) error) error {
	q := goqu.Select().From(r.tableName).Where(goqu.C(r.idColumn).Eq(id))
	return queryFunc(ctx, q, false, f)
}

func (r *repository) destroyExisting(ctx context.Context, ids []int) error {
	for _, id := range ids {
		exists, err := r.exists(ctx, id)
		if err != nil {
			return err
		}

		if !exists {
			return fmt.Errorf("%s %d does not exist in %s", r.idColumn, id, r.tableName)
		}
	}

	return r.destroy(ctx, ids)
}

func (r *repository) destroy(ctx context.Context, ids []int) error {
	for _, id := range ids {
		q := goqu.Delete(r.tableName).Where(goqu.C(r.idColumn).Eq(id))
		if _, err := destroy(ctx, q); err != nil {
			return err
		}
	}

	return nil
}

func (r *repository) exists(ctx context.Context, id int) (bool, error) {
	q := goqu.Select(goqu.COUNT("*")).From(r.tableName).Where(goqu.C(r.idColumn).Eq(id))

	c, err := queryInt(ctx, q)
	if err != nil {
		return false, err
	}

	return c > 0, nil
}

func (r *repository) buildCountQuery(query string) string {
	return "SELECT COUNT(*) FROM (" + query + ") as temp"
}

// query executes a query, using sql.Exec
func (r *repository) exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	db, err := getDBWrapper(ctx)
	if err != nil {
		return nil, err
	}

	return db.tx.Exec(ctx, query, args...)
}

// query prepares a query, using sql.Prepare
// func (r *repository) queryPrepare(ctx context.Context, query string) (*stmt, error) {
// 	db, err := getDBWrapper(ctx)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	return db.tx.Prepare(ctx, query)
// }

// query runs a query returning a single row, using sqlx.Get
func (r *repository) querySingle(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	db, err := getDBWrapper(ctx)
	if err != nil {
		return err
	}

	return db.tx.Get(ctx, dest, query, args...)
}

// query runs a query returning multiple rows, using sqlx.Select
// func (r *repository) query(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
// 	db, err := getDBWrapper(ctx)
// 	if err != nil {
// 		return err
// 	}
//
// 	return db.tx.Select(ctx, dest, query, args...)
// }

// query runs a query returning a single integer value, using sqlx.Get
func (r *repository) queryInt(ctx context.Context, query string, args ...interface{}) (int, error) {
	db, err := getDBWrapper(ctx)
	if err != nil {
		return 0, err
	}

	return db.tx.GetInt(ctx, query, args...)
}

// query runs a query returning multiple rows, each with a singular integer value, using sqlx.Select
func (r *repository) queryInts(ctx context.Context, query string, args ...interface{}) ([]int, error) {
	db, err := getDBWrapper(ctx)
	if err != nil {
		return nil, err
	}

	return db.tx.GetInts(ctx, query, args...)
}

// query runs a query returning a single or multiple rows, running f for each returned row
func (r *repository) queryFunc(ctx context.Context, query string, args []interface{}, single bool, f func(rows *sqlx.Rows) error) error {
	db, err := getDBWrapper(ctx)
	if err != nil {
		return err
	}

	return db.tx.QueryFunc(ctx, query, args, single, f)
}

func (r *repository) buildQueryBody(body string, whereClauses []string, havingClauses []string) string {
	if len(whereClauses) > 0 {
		body = body + " WHERE " + strings.Join(whereClauses, " AND ") // TODO handle AND or OR
	}
	if len(havingClauses) > 0 {
		body = body + " GROUP BY " + r.tableName + ".id "
		body = body + " HAVING " + strings.Join(havingClauses, " AND ") // TODO handle AND or OR
	}

	return body
}

func (r *repository) executeFindQuery(ctx context.Context, body string, args []interface{}, sortAndPagination string, whereClauses []string, havingClauses []string, withClauses []string, recursiveWith bool) ([]int, int, error) {
	body = r.buildQueryBody(body, whereClauses, havingClauses)

	withClause := ""
	if len(withClauses) > 0 {
		var recursive string
		if recursiveWith {
			recursive = " RECURSIVE "
		}
		withClause = "WITH " + recursive + strings.Join(withClauses, ", ") + " "
	}

	countQuery := withClause + r.buildCountQuery(body)
	idsQuery := withClause + body + sortAndPagination

	// Perform query and fetch result

	countResult, err := r.queryInt(ctx, countQuery, args...)
	if err != nil {
		return nil, 0, err
	}

	idsResult, err := r.queryInts(ctx, idsQuery, args...)
	if err != nil {
		return nil, 0, err
	}

	return idsResult, countResult, nil
}

func (r *repository) newQuery() queryBuilder {
	return queryBuilder{
		repository: r,
	}
}

func (r *repository) join(j joiner, as string, parentIDCol string) {
	t := r.tableName
	if as != "" {
		t = as
	}
	j.addLeftJoin(r.tableName, as, fmt.Sprintf("%s.%s = %s", t, r.idColumn, parentIDCol))
}

//nolint:golint,unused
func (r *repository) innerJoin(j joiner, as string, parentIDCol string) {
	t := r.tableName
	if as != "" {
		t = as
	}
	j.addInnerJoin(r.tableName, as, fmt.Sprintf("%s.%s = %s", t, r.idColumn, parentIDCol))
}

type joiner interface {
	addLeftJoin(table, as, onClause string)
	addInnerJoin(table, as, onClause string)
}

type joinRepository struct {
	repository
	fkColumn string

	// fields for ordering
	foreignTable string
	orderExp     exp.OrderedExpression
}

func (r *joinRepository) getIDs(ctx context.Context, id int) ([]int, error) {
	table := goqu.T(r.tableName)
	q := goqu.Select(table.Col(r.fkColumn)).From(table)
	if r.foreignTable != "" {
		fTable := goqu.T(r.foreignTable)
		q = q.InnerJoin(fTable, goqu.On(fTable.Col("id").Eq(table.Col(r.fkColumn))))
	}

	q = q.Where(goqu.C(r.idColumn).Eq(id))

	if r.orderExp != nil {
		q = q.Order(r.orderExp)
	}

	return queryInts(ctx, q)
}

func (r *joinRepository) insert(ctx context.Context, id int, foreignIDs ...int) error {
	db, err := getDBWrapper(ctx)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("INSERT INTO %s (%s, %s) VALUES ($1, $2)", r.tableName, r.idColumn, r.fkColumn)
	stmt, err := db.tx.Prepare(ctx, query)
	if err != nil {
		return err
	}

	defer stmt.Close()

	for _, fk := range foreignIDs {
		if _, err := stmt.Exec(ctx, id, fk); err != nil {
			return err
		}
	}
	return nil
}

// insertOrIgnore inserts a join into the table, silently failing in the event that a conflict occurs (ie when the join already exists)
func (r *joinRepository) insertOrIgnore(ctx context.Context, id int, foreignIDs []int) error {
	db, err := getDBWrapper(ctx)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("INSERT INTO %s (%s, %s) VALUES ($1, $2) ON CONFLICT (%[2]s, %s) DO NOTHING", r.tableName, r.idColumn, r.fkColumn)
	stmt, err := db.tx.Prepare(ctx, query)
	if err != nil {
		return err
	}

	defer stmt.Close()

	for _, fk := range foreignIDs {
		if _, err := stmt.Exec(ctx, id, fk); err != nil {
			return err
		}
	}
	return nil
}

func (r *joinRepository) destroyJoins(ctx context.Context, id int, foreignIDs []int) error {
	if len(foreignIDs) == 0 {
		return nil
	}

	q := goqu.Delete(r.tableName).Where(goqu.C(r.idColumn).Eq(id), goqu.C(r.fkColumn).In(foreignIDs))
	_, err := destroy(ctx, q)
	return err
}

func (r *joinRepository) replace(ctx context.Context, id int, foreignIDs []int) error {
	if err := r.destroy(ctx, []int{id}); err != nil {
		return err
	}

	for _, fk := range foreignIDs {
		if err := r.insert(ctx, id, fk); err != nil {
			return err
		}
	}

	return nil
}

type captionRepository struct {
	repository
}

func (r *captionRepository) get(ctx context.Context, id models.FileID) ([]*models.VideoCaption, error) {
	query := fmt.Sprintf("SELECT %s, %s, %s from %s WHERE %s = ?", captionCodeColumn, captionFilenameColumn, captionTypeColumn, r.tableName, r.idColumn)
	var ret []*models.VideoCaption
	err := r.queryFunc(ctx, query, []interface{}{id}, false, func(rows *sqlx.Rows) error {
		var captionCode string
		var captionFilename string
		var captionType string

		if err := rows.Scan(&captionCode, &captionFilename, &captionType); err != nil {
			return err
		}

		caption := &models.VideoCaption{
			LanguageCode: captionCode,
			Filename:     captionFilename,
			CaptionType:  captionType,
		}
		ret = append(ret, caption)
		return nil
	})
	return ret, err
}

func (r *captionRepository) insert(ctx context.Context, id models.FileID, caption *models.VideoCaption) (sql.Result, error) {
	row := goqu.Record{}
	row[r.idColumn] = id
	row[captionCodeColumn] = caption.LanguageCode
	row[captionFilenameColumn] = caption.Filename
	row[captionTypeColumn] = caption.CaptionType

	q := goqu.Insert(r.tableName).Prepared(true).Rows(row)
	return insert(ctx, q)
}

func (r *captionRepository) replace(ctx context.Context, id models.FileID, captions []*models.VideoCaption) error {
	if err := r.destroy(ctx, []int{int(id)}); err != nil {
		return err
	}

	for _, caption := range captions {
		if _, err := r.insert(ctx, id, caption); err != nil {
			return err
		}
	}

	return nil
}

type stringRepository struct {
	repository
	stringColumn string
}

func (r *stringRepository) get(ctx context.Context, id int) ([]string, error) {
	q := goqu.Select(goqu.C(r.stringColumn)).From(goqu.T(r.tableName)).Where(goqu.C(r.idColumn).Eq(id))

	var ret []string
	err := querySelect(ctx, &ret, q)
	return ret, err
}

func (r *stringRepository) insert(ctx context.Context, id int, s string) (sql.Result, error) {
	row := goqu.Record{}
	row[r.idColumn] = id
	row[r.stringColumn] = s

	q := goqu.Insert(r.tableName).Prepared(true).Rows(row)
	return insert(ctx, q)
}

func (r *stringRepository) replace(ctx context.Context, id int, newStrings []string) error {
	if err := r.destroy(ctx, []int{id}); err != nil {
		return err
	}

	for _, s := range newStrings {
		if _, err := r.insert(ctx, id, s); err != nil {
			return err
		}
	}

	return nil
}

type stashIDRepository struct {
	repository
}

func (r *stashIDRepository) get(ctx context.Context, id int) ([]models.StashID, error) {
	q := goqu.Select(goqu.C("stash_id"), goqu.C("endpoint")).From(goqu.T(r.tableName)).Where(goqu.C(r.idColumn).Eq(id))

	var ret []models.StashID
	err := querySelect(ctx, &ret, q)
	return ret, err
}

func (r *stashIDRepository) replace(ctx context.Context, id int, newIDs []models.StashID) error {
	if err := r.destroy(ctx, []int{id}); err != nil {
		return err
	}

	if len(newIDs) == 0 {
		return nil
	}

	var vals [][]interface{}
	for _, stashID := range newIDs {
		vals = append(vals, goqu.Vals{id, stashID.Endpoint, stashID.StashID})
	}

	q := goqu.Insert(goqu.T(r.tableName)).Prepared(true).Cols(r.idColumn, "endpoint", "stash_id").Vals(vals...)
	_, err := insert(ctx, q)
	if err != nil {
		return err
	}

	return nil
}

type filesRepository struct {
	repository
}

type relatedFileRow struct {
	ID      int  `db:"id"`
	FileID  int  `db:"file_id"`
	Primary bool `db:"primary"`
}

func (r *filesRepository) getMany(ctx context.Context, ids []int, primaryOnly bool) ([][]models.FileID, error) {
	if len(ids) == 0 {
		return [][]models.FileID{}, nil
	}

	q := goqu.Select(goqu.C(r.idColumn).As("id"), "file_id", "primary").From(r.tableName).Where(goqu.C(r.idColumn).In(ids))

	if primaryOnly {
		q = q.Where(goqu.C("primary").Eq(true))
	}

	var fileRows []relatedFileRow
	err := querySelect(ctx, &fileRows, q)
	if err != nil {
		return nil, err
	}

	ret := make([][]models.FileID, len(ids))
	idToIndex := make(map[int]int)
	for i, id := range ids {
		idToIndex[id] = i
	}

	for _, row := range fileRows {
		id := row.ID
		fileID := models.FileID(row.FileID)

		if row.Primary {
			// prepend to list
			ret[idToIndex[id]] = append([]models.FileID{fileID}, ret[idToIndex[id]]...)
		} else {
			ret[idToIndex[id]] = append(ret[idToIndex[id]], fileID)
		}
	}

	return ret, nil
}

func (r *filesRepository) get(ctx context.Context, id int) ([]models.FileID, error) {
	// ORDER BY primary DESC to sort primary file first
	q := goqu.Select("file_id").From(r.tableName).Where(goqu.C(r.idColumn).Eq(id)).Order(goqu.C("primary").Desc())

	var ret []models.FileID
	err := querySelect(ctx, &ret, q)
	return ret, err
}
