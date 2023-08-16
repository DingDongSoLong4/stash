package migrations_sqlite

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/stashapp/stash/pkg/logger"
)

func pre49(ctx context.Context, db *sqlx.DB) error {
	logger.Info("Running pre-migration for schema version 49")

	m := schema49PreMigrator{
		migrator: migrator{
			db: db,
		},
	}

	if err := m.stringifyFingerprints(ctx); err != nil {
		return err
	}

	return nil
}

type schema49PreMigrator struct {
	migrator
}

func (m *schema49PreMigrator) stringifyFingerprints(ctx context.Context) error {
	const (
		limit    = 1000
		logEvery = 10000
	)

	var total int

	err := m.db.Get(&total, `SELECT COUNT(*) FROM "files_fingerprints" WHERE type = 'phash' AND typeof(fingerprint) = 'integer'`)
	if err != nil {
		return err
	}

	if total == 0 {
		return nil
	}

	logger.Infof("Migrating %d phashes...", total)

	lastID := 0
	count := 0

	for {
		gotSome := false

		if err := m.withTxn(ctx, func(tx *sqlx.Tx) error {
			query := `SELECT "file_id", "type", "fingerprint" FROM "files_fingerprints" WHERE type = 'phash' AND typeof("fingerprint") = 'integer' `
			if lastID != 0 {
				query += fmt.Sprintf(`AND "file_id" > %d `, lastID)
			}

			query += fmt.Sprintf(`ORDER BY "file_id" LIMIT %d`, limit)

			rows, err := m.db.Query(query)
			if err != nil {
				return err
			}
			defer rows.Close()

			stmt, err := tx.Prepare(`UPDATE "files_fingerprints" SET "fingerprint" = ? WHERE "file_id" = ? AND "type" = ? AND "fingerprint" = ?`)
			if err != nil {
				return err
			}
			defer stmt.Close()

			for rows.Next() {
				var id int
				var fpType string
				var fp int64

				err := rows.Scan(&id, &fpType, &fp)
				if err != nil {
					return err
				}

				gotSome = true
				lastID = id
				count++

				fpStr := fmt.Sprintf("%016x", uint64(fp))

				_, err = stmt.Exec(fpStr, id, fpType, fp)
				if err != nil {
					return err
				}
			}

			return rows.Err()
		}); err != nil {
			return err
		}

		if !gotSome {
			break
		}

		if count%logEvery == 0 {
			logger.Infof("Migrated %d phashes", count)
		}
	}

	return nil
}
