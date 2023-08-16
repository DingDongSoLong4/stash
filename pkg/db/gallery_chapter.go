package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jmoiron/sqlx"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sliceutil/intslice"
)

const (
	galleriesChaptersTable = "galleries_chapters"
)

type galleryChapterRow struct {
	ID         int       `db:"id" goqu:"skipinsert"`
	Title      string    `db:"title"`
	ImageIndex int       `db:"image_index"`
	GalleryID  int       `db:"gallery_id"`
	CreatedAt  Timestamp `db:"created_at"`
	UpdatedAt  Timestamp `db:"updated_at"`
}

func (r *galleryChapterRow) fromGalleryChapter(o models.GalleryChapter) {
	r.ID = o.ID
	r.Title = o.Title
	r.ImageIndex = o.ImageIndex
	r.GalleryID = o.GalleryID
	r.CreatedAt = Timestamp{Timestamp: o.CreatedAt}
	r.UpdatedAt = Timestamp{Timestamp: o.UpdatedAt}
}

func (r *galleryChapterRow) resolve() *models.GalleryChapter {
	ret := &models.GalleryChapter{
		ID:         r.ID,
		Title:      r.Title,
		ImageIndex: r.ImageIndex,
		GalleryID:  r.GalleryID,
		CreatedAt:  r.CreatedAt.Timestamp,
		UpdatedAt:  r.UpdatedAt.Timestamp,
	}

	return ret
}

type GalleryChapterStore struct {
	repository

	tableMgr *table
}

func NewGalleryChapterStore() *GalleryChapterStore {
	return &GalleryChapterStore{
		repository: repository{
			tableName: galleriesChaptersTable,
			idColumn:  idColumn,
		},
		tableMgr: galleriesChaptersTableMgr,
	}
}

func (qb *GalleryChapterStore) table() exp.IdentifierExpression {
	return qb.tableMgr.table
}

func (qb *GalleryChapterStore) selectDataset() *goqu.SelectDataset {
	return goqu.From(qb.table()).Select(qb.table().All())
}

func (qb *GalleryChapterStore) Create(ctx context.Context, newObject *models.GalleryChapter) error {
	var r galleryChapterRow
	r.fromGalleryChapter(*newObject)

	id, err := qb.tableMgr.insertID(ctx, r)
	if err != nil {
		return err
	}

	updated, err := qb.find(ctx, id)
	if err != nil {
		return fmt.Errorf("finding after create: %w", err)
	}

	*newObject = *updated

	return nil
}

func (qb *GalleryChapterStore) Update(ctx context.Context, updatedObject *models.GalleryChapter) error {
	var r galleryChapterRow
	r.fromGalleryChapter(*updatedObject)

	if err := qb.tableMgr.updateByID(ctx, updatedObject.ID, r); err != nil {
		return err
	}

	return nil
}

func (qb *GalleryChapterStore) Destroy(ctx context.Context, id int) error {
	return qb.destroyExisting(ctx, []int{id})
}

// returns nil, nil if not found
func (qb *GalleryChapterStore) Find(ctx context.Context, id int) (*models.GalleryChapter, error) {
	ret, err := qb.find(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return ret, err
}

func (qb *GalleryChapterStore) FindMany(ctx context.Context, ids []int) ([]*models.GalleryChapter, error) {
	ret := make([]*models.GalleryChapter, len(ids))

	if len(ids) == 0 {
		return ret, nil
	}

	table := qb.table()
	q := qb.selectDataset().Prepared(true).Where(table.Col(idColumn).In(ids))
	unsorted, err := qb.getMany(ctx, q)
	if err != nil {
		return nil, err
	}

	for _, s := range unsorted {
		i := intslice.IntIndex(ids, s.ID)
		ret[i] = s
	}

	for i := range ret {
		if ret[i] == nil {
			return nil, fmt.Errorf("gallery chapter with id %d not found", ids[i])
		}
	}

	return ret, nil
}

// returns nil, sql.ErrNoRows if not found
func (qb *GalleryChapterStore) find(ctx context.Context, id int) (*models.GalleryChapter, error) {
	q := qb.selectDataset().Where(qb.tableMgr.byID(id))

	ret, err := qb.get(ctx, q)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// returns nil, sql.ErrNoRows if not found
func (qb *GalleryChapterStore) get(ctx context.Context, q *goqu.SelectDataset) (*models.GalleryChapter, error) {
	ret, err := qb.getMany(ctx, q)
	if err != nil {
		return nil, err
	}

	if len(ret) == 0 {
		return nil, sql.ErrNoRows
	}

	return ret[0], nil
}

func (qb *GalleryChapterStore) getMany(ctx context.Context, q *goqu.SelectDataset) ([]*models.GalleryChapter, error) {
	const single = false
	var ret []*models.GalleryChapter
	if err := queryFunc(ctx, q, single, func(r *sqlx.Rows) error {
		var f galleryChapterRow
		if err := r.StructScan(&f); err != nil {
			return err
		}

		s := f.resolve()

		ret = append(ret, s)
		return nil
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (qb *GalleryChapterStore) FindByGalleryID(ctx context.Context, galleryID int) ([]*models.GalleryChapter, error) {
	table := qb.table()
	q := qb.selectDataset().Where(table.Col("gallery_id").Eq(galleryID)).
		GroupBy(table.Col(idColumn)).Order(table.Col("image_index").Asc())
	return qb.getMany(ctx, q)
}
