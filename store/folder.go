package store

import (
	"context"
	"errors"
)

// Folder errors surfaced by every driver.
var (
	// ErrFolderNotFound indicates a missing folder.
	ErrFolderNotFound = errors.New("folder not found")
	// ErrFolderPermissionDenied indicates that the actor does not own the folder.
	ErrFolderPermissionDenied = errors.New("folder permission denied")
	// ErrFolderAlreadyExists indicates a duplicate immutable folder ID.
	ErrFolderAlreadyExists = errors.New("folder already exists")
)

// Folder is a node in a user's personal folder tree. Folders are never shared:
// every folder has exactly one creator, and only that creator can read or
// mutate it.
type Folder struct {
	ID        int32
	UID       string
	CreatorID int32
	// ParentID is nil for a root folder.
	ParentID  *int32
	Name      string
	CreatedTs int64
	UpdatedTs int64
}

// FindFolder selects folders.
type FindFolder struct {
	ID        *int32
	UID       *string
	CreatorID *int32
	// ParentID selects folders directly below one folder.
	ParentID *int32
	// RootOnly selects folders without a parent.
	RootOnly bool

	// Pagination
	Limit  *int
	Offset *int
}

// UpdateFolder contains mutable folder fields. ActorUserID is the caller whose
// ownership the driver revalidates in the write transaction.
type UpdateFolder struct {
	ID          int32
	ActorUserID int32
	Name        *string
	ParentID    *int32
	ClearParent bool
}

// DeleteFolder identifies a folder to delete. The driver deletes every
// descendant folder and clears the folder of every memo filed in the subtree;
// the memos themselves are kept. ActorUserID is the caller whose ownership the
// driver revalidates in the write transaction.
type DeleteFolder struct {
	ID          int32
	ActorUserID int32
}

// CreateFolder creates a folder owned by create.CreatorID.
func (s *Store) CreateFolder(ctx context.Context, create *Folder) (*Folder, error) {
	return s.driver.CreateFolder(ctx, create)
}

// ListFolders lists folders matching find.
func (s *Store) ListFolders(ctx context.Context, find *FindFolder) ([]*Folder, error) {
	return s.driver.ListFolders(ctx, find)
}

// GetFolder returns the first folder matching find, or nil when none exists.
func (s *Store) GetFolder(ctx context.Context, find *FindFolder) (*Folder, error) {
	folders, err := s.driver.ListFolders(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(folders) == 0 {
		return nil, nil
	}
	return folders[0], nil
}

// UpdateFolder updates a folder and returns its stored state.
func (s *Store) UpdateFolder(ctx context.Context, update *UpdateFolder) (*Folder, error) {
	return s.driver.UpdateFolder(ctx, update)
}

// DeleteFolder deletes a folder subtree and detaches its memos.
func (s *Store) DeleteFolder(ctx context.Context, delete *DeleteFolder) error {
	return s.driver.DeleteFolder(ctx, delete)
}
