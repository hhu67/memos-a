package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateFolder(ctx context.Context, create *store.Folder) (*store.Folder, error) {
	fields := []string{"uid", "creator_id", "parent_id", "name"}
	args := []any{create.UID, create.CreatorID, create.ParentID, create.Name}
	if create.CreatedTs != 0 {
		fields = append(fields, "created_ts")
		args = append(args, create.CreatedTs)
	}
	if create.UpdatedTs != 0 {
		fields = append(fields, "updated_ts")
		args = append(args, create.UpdatedTs)
	}
	query := "INSERT INTO folder (" + strings.Join(fields, ", ") + ") VALUES (" + placeholders(len(args)) +
		") RETURNING id, uid, creator_id, parent_id, name, created_ts, updated_ts"
	folder := &store.Folder{}
	if err := d.db.QueryRowContext(ctx, query, args...).Scan(
		&folder.ID, &folder.UID, &folder.CreatorID, &folder.ParentID, &folder.Name, &folder.CreatedTs, &folder.UpdatedTs,
	); err != nil {
		if isPostgresUniqueViolation(err) {
			return nil, store.ErrFolderAlreadyExists
		}
		return nil, errors.Wrap(err, "failed to create folder")
	}
	return folder, nil
}

func (d *DB) ListFolders(ctx context.Context, find *store.FindFolder) ([]*store.Folder, error) {
	where, args := []string{"1 = 1"}, []any{}
	if find.ID != nil {
		where, args = append(where, "folder.id = "+placeholder(len(args)+1)), append(args, *find.ID)
	}
	if find.UID != nil {
		where, args = append(where, "folder.uid = "+placeholder(len(args)+1)), append(args, *find.UID)
	}
	if find.CreatorID != nil {
		where, args = append(where, "folder.creator_id = "+placeholder(len(args)+1)), append(args, *find.CreatorID)
	}
	if find.ParentID != nil {
		where, args = append(where, "folder.parent_id = "+placeholder(len(args)+1)), append(args, *find.ParentID)
	}
	if find.RootOnly {
		where = append(where, "folder.parent_id IS NULL")
	}
	query := "SELECT id, uid, creator_id, parent_id, name, created_ts, updated_ts FROM folder WHERE " +
		strings.Join(where, " AND ") + " ORDER BY name ASC, id ASC"
	if find.Limit != nil {
		query = fmt.Sprintf("%s LIMIT %d", query, *find.Limit)
		if find.Offset != nil {
			query = fmt.Sprintf("%s OFFSET %d", query, *find.Offset)
		}
	}
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
	var creatorID int32
	if err := tx.QueryRowContext(ctx, "SELECT creator_id FROM folder WHERE id = "+placeholder(1), update.ID).Scan(&creatorID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrFolderNotFound
		}
		return nil, err
	}
	if update.ActorUserID != 0 && creatorID != update.ActorUserID {
		return nil, store.ErrFolderPermissionDenied
	}
	sets, args := []string{}, []any{}
	if update.Name != nil {
		args = append(args, *update.Name)
		sets = append(sets, "name = "+placeholder(len(args)))
	}
	if update.ClearParent {
		sets = append(sets, "parent_id = NULL")
	} else if update.ParentID != nil {
		args = append(args, *update.ParentID)
		sets = append(sets, "parent_id = "+placeholder(len(args)))
	}
	sets = append(sets, "updated_ts = EXTRACT(EPOCH FROM NOW())")
	args = append(args, update.ID)
	query := "UPDATE folder SET " + strings.Join(sets, ", ") + " WHERE id = " + placeholder(len(args)) +
		" RETURNING id, uid, creator_id, parent_id, name, created_ts, updated_ts"
	folder := &store.Folder{}
	if err := tx.QueryRowContext(ctx, query, args...).Scan(
		&folder.ID, &folder.UID, &folder.CreatorID, &folder.ParentID, &folder.Name, &folder.CreatedTs, &folder.UpdatedTs,
	); err != nil {
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
	var creatorID int32
	if err := tx.QueryRowContext(ctx, "SELECT creator_id FROM folder WHERE id = "+placeholder(1), delete.ID).Scan(&creatorID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.ErrFolderNotFound
		}
		return err
	}
	if delete.ActorUserID != 0 && creatorID != delete.ActorUserID {
		return store.ErrFolderPermissionDenied
	}
	ids := []int32{delete.ID}
	frontier := []int32{delete.ID}
	for len(frontier) > 0 {
		args := make([]any, 0, len(frontier))
		for _, id := range frontier {
			args = append(args, id)
		}
		rows, err := tx.QueryContext(ctx, "SELECT id FROM folder WHERE parent_id IN ("+placeholders(len(args))+")", args...)
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
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	list := placeholders(len(args))
	if _, err := tx.ExecContext(ctx, "UPDATE memo SET folder_id = NULL WHERE folder_id IN ("+list+")", args...); err != nil {
		return errors.Wrap(err, "failed to detach memos from deleted folders")
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM folder WHERE id IN ("+list+")", args...); err != nil {
		return errors.Wrap(err, "failed to delete folders")
	}
	return tx.Commit()
}
