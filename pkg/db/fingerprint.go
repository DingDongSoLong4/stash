package db

import (
	"context"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/stashapp/stash/pkg/models"
)

const (
	fingerprintTable = "files_fingerprints"
)

type fingerprintQueryRow struct {
	Type        string `db:"fingerprint_type"`
	Fingerprint string `db:"fingerprint"`
}

func (r fingerprintQueryRow) valid() bool {
	return r.Type != "" && r.Fingerprint != ""
}

func (r *fingerprintQueryRow) resolve() models.Fingerprint {
	return models.Fingerprint{
		Type:        r.Type,
		Fingerprint: r.Fingerprint,
	}
}

type fingerprintQueryBuilder struct {
	repository

	tableMgr *table
}

var FingerprintReaderWriter = &fingerprintQueryBuilder{
	repository: repository{
		tableName: fingerprintTable,
		idColumn:  fileIDColumn,
	},

	tableMgr: fingerprintTableMgr,
}

func (qb *fingerprintQueryBuilder) insert(ctx context.Context, fileID models.FileID, f models.Fingerprint) error {
	table := qb.table()
	q := goqu.Insert(table).Cols(fileIDColumn, "type", "fingerprint").Vals(
		goqu.Vals{fileID, f.Type, f.Fingerprint},
	)
	_, err := insert(ctx, q)
	if err != nil {
		return fmt.Errorf("inserting into %s: %w", table.GetTable(), err)
	}

	return nil
}

func (qb *fingerprintQueryBuilder) insertJoins(ctx context.Context, fileID models.FileID, f []models.Fingerprint) error {
	for _, ff := range f {
		if err := qb.insert(ctx, fileID, ff); err != nil {
			return err
		}
	}

	return nil
}

func (qb *fingerprintQueryBuilder) replaceJoins(ctx context.Context, fileID models.FileID, f []models.Fingerprint) error {
	if err := qb.destroy(ctx, []int{int(fileID)}); err != nil {
		return err
	}

	return qb.insertJoins(ctx, fileID, f)
}

func (qb *fingerprintQueryBuilder) table() exp.IdentifierExpression {
	return qb.tableMgr.table
}
