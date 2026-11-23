package sqlite

import (
	"context"
	"database/sql"
	"strings"

	"github.com/pkg/errors"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateFolder(ctx context.Context, create *store.Folder) (*store.Folder, error) {
	fields := []string{"`uid`", "`creator_id`", "`parent_id`", "`name`"}
	placeholders := []string{"?", "?", "?", "?"}
	args := []any{create.UID, create.CreatorID, create.ParentID, create.Name}
	if create.CreatedTs != 0 {
		fields = append(fields, "`created_ts`")
		placeholders = append(placeholders, "?")
		args = append(args, create.CreatedTs)
	}
	if create.UpdatedTs != 0 {
		fields = append(fields, "`updated_ts`")
		placeholders = append(placeholders, "?")
		args = append(args, create.UpdatedTs)
	}
	query := "INSERT INTO `folder` (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ") RETURNING `id`, `created_ts`, `updated_ts`"
	folder := *create
	if err := d.db.QueryRowContext(ctx, query, args...).Scan(&folder.ID, &folder.CreatedTs, &folder.UpdatedTs); err != nil {
		if isSQLiteUniqueViolation(err) {
			return nil, store.ErrFolderAlreadyExists
		}
		return nil, errors.Wrap(err, "failed to create folder")
	}
	return &folder, nil
}

func (d *DB) ListFolders(ctx context.Context, find *store.FindFolder) ([]*store.Folder, error) {
	where, args := []string{"1 = 1"}, []any{}
	if find.ID != nil {
		where, args = append(where, "`folder`.`id` = ?"), append(args, *find.ID)
	}
	if find.UID != nil {
		where, args = append(where, "`folder`.`uid` = ?"), append(args, *find.UID)
	}
	if find.CreatorID != nil {
		where, args = append(where, "`folder`.`creator_id` = ?"), append(args, *find.CreatorID)
	}
	if find.ParentID != nil {
		where, args = append(where, "`folder`.`parent_id` = ?"), append(args, *find.ParentID)
	}
	if find.RootOnly {
		where = append(where, "`folder`.`parent_id` IS NULL")
	}
	query := "SELECT `id`, `uid`, `creator_id`, `parent_id`, `name`, `created_ts`, `updated_ts` FROM `folder` WHERE " +
		strings.Join(where, " AND ") + " ORDER BY `name` ASC, `id` ASC"
	query = appendSQLiteLimit(query, find.Limit, find.Offset)
	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	folders := []*store.Folder{}
	for rows.Next() {
		folder := &store.Folder{}
		if err := rows.Scan(&folder.ID, &folder.UID, &folder.CreatorID, &folder.ParentID, &folder.Name, &folder.CreatedTs, &folder.UpdatedTs); err != nil {
			return nil, err
		}
		folders = append(folders, folder)
	}
	return folders, rows.Err()
}

func (d *DB) UpdateFolder(ctx context.Context, update *store.UpdateFolder) (*store.Folder, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	creatorID, err := sqliteFolderCreator(ctx, tx, update.ID)
	if err != nil {
		return nil, err
	}
	if update.ActorUserID != 0 && creatorID != update.ActorUserID {
		return nil, store.ErrFolderPermissionDenied
	}
	sets, args := []string{}, []any{}
	if update.Name != nil {
		sets, args = append(sets, "`name` = ?"), append(args, *update.Name)
	}
	if update.ClearParent {
		sets = append(sets, "`parent_id` = NULL")
	} else if update.ParentID != nil {
		sets, args = append(sets, "`parent_id` = ?"), append(args, *update.ParentID)
	}
	sets = append(sets, "`updated_ts` = strftime('%s', 'now')")
	args = append(args, update.ID)
	query := "UPDATE `folder` SET " + strings.Join(sets, ", ") +
		" WHERE `id` = ? RETURNING `id`, `uid`, `creator_id`, `parent_id`, `name`, `created_ts`, `updated_ts`"
	folder := &store.Folder{}
	if err := tx.QueryRowContext(ctx, query, args...).Scan(
		&folder.ID, &folder.UID, &folder.CreatorID, &folder.ParentID, &folder.Name, &folder.CreatedTs, &folder.UpdatedTs,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrFolderNotFound
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return folder, nil
}

func (d *DB) DeleteFolder(ctx context.Context, delete *store.DeleteFolder) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	creatorID, err := sqliteFolderCreator(ctx, tx, delete.ID)
	if err != nil {
		return err
	}
	if delete.ActorUserID != 0 && creatorID != delete.ActorUserID {
		return store.ErrFolderPermissionDenied
	}
	// Collect the subtree breadth-first, so the same portable SQL runs on every
	// driver and a malformed tree cannot hide a row from the delete.
	ids := []int32{delete.ID}
	frontier := []int32{delete.ID}
	for len(frontier) > 0 {
		placeholders := make([]string, 0, len(frontier))
		args := make([]any, 0, len(frontier))
		for _, id := range frontier {
			placeholders = append(placeholders, "?")
			args = append(args, id)
		}
		rows, err := tx.QueryContext(ctx, "SELECT `id` FROM `folder` WHERE `parent_id` IN ("+strings.Join(placeholders, ", ")+")", args...)
		if err != nil {
			return err
		}
		next := []int32{}
		for rows.Next() {
			var id int32
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return err
			}
			next = append(next, id)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		_ = rows.Close()
		ids = append(ids, next...)
		frontier = next
	}
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	list := strings.Join(placeholders, ", ")
	if _, err := tx.ExecContext(ctx, "UPDATE `memo` SET `folder_id` = NULL WHERE `folder_id` IN ("+list+")", args...); err != nil {
		return errors.Wrap(err, "failed to detach memos from deleted folders")
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM `folder` WHERE `id` IN ("+list+")", args...); err != nil {
		return errors.Wrap(err, "failed to delete folders")
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func sqliteFolderCreator(ctx context.Context, tx *sql.Tx, id int32) (int32, error) {
	var creatorID int32
	if err := tx.QueryRowContext(ctx, "SELECT `creator_id` FROM `folder` WHERE `id` = ?", id).Scan(&creatorID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, store.ErrFolderNotFound
		}
		return 0, err
	}
	return creatorID, nil
}
