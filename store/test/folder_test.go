package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/usememos/memos/store"
)

func TestFolderStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()
	user, err := createTestingHostUser(ctx, ts)
	require.NoError(t, err)

	root, err := ts.CreateFolder(ctx, &store.Folder{
		UID:       "folder-root",
		CreatorID: user.ID,
		Name:      "Root",
	})
	require.NoError(t, err)
	require.NotZero(t, root.ID)
	require.Nil(t, root.ParentID)

	child, err := ts.CreateFolder(ctx, &store.Folder{
		UID:       "folder-child",
		CreatorID: user.ID,
		ParentID:  &root.ID,
		Name:      "Child",
	})
	require.NoError(t, err)
	require.NotNil(t, child.ParentID)
	require.Equal(t, root.ID, *child.ParentID)

	folders, err := ts.ListFolders(ctx, &store.FindFolder{CreatorID: &user.ID})
	require.NoError(t, err)
	require.Len(t, folders, 2)

	children, err := ts.ListFolders(ctx, &store.FindFolder{CreatorID: &user.ID, ParentID: &root.ID})
	require.NoError(t, err)
	require.Len(t, children, 1)
	require.Equal(t, child.ID, children[0].ID)

	renamed := "Renamed root"
	root, err = ts.UpdateFolder(ctx, &store.UpdateFolder{ID: root.ID, ActorUserID: user.ID, Name: &renamed})
	require.NoError(t, err)
	require.Equal(t, renamed, root.Name)

	// A memo keeps its title and folder through a round trip, can be detached,
	// and is found by title.
	memo, err := ts.CreateMemo(ctx, &store.Memo{
		UID:        "foldered-memo",
		CreatorID:  user.ID,
		Content:    "body",
		Title:      "Plan",
		FolderID:   &child.ID,
		Visibility: store.Private,
	})
	require.NoError(t, err)
	require.Equal(t, "Plan", memo.Title)
	require.NotNil(t, memo.FolderID)
	require.Equal(t, child.ID, *memo.FolderID)

	found, err := ts.GetMemo(ctx, &store.FindMemo{UID: &memo.UID})
	require.NoError(t, err)
	require.Equal(t, "Plan", found.Title)
	require.NotNil(t, found.FolderID)
	require.Equal(t, child.ID, *found.FolderID)

	inFolder, err := ts.ListMemos(ctx, &store.FindMemo{CreatorID: &user.ID, FolderID: &child.ID})
	require.NoError(t, err)
	require.Len(t, inFolder, 1)
	require.Equal(t, memo.ID, inFolder[0].ID)

	byTitle, err := ts.ListMemos(ctx, &store.FindMemo{CreatorID: &user.ID, TitleSearch: "lan"})
	require.NoError(t, err)
	require.Len(t, byTitle, 1)
	require.Equal(t, memo.ID, byTitle[0].ID)

	// Deleting the root removes the whole subtree and detaches the memo, which
	// survives in the root.
	require.NoError(t, ts.DeleteFolder(ctx, &store.DeleteFolder{ID: root.ID, ActorUserID: user.ID}))
	folders, err = ts.ListFolders(ctx, &store.FindFolder{CreatorID: &user.ID})
	require.NoError(t, err)
	require.Empty(t, folders)

	found, err = ts.GetMemo(ctx, &store.FindMemo{UID: &memo.UID})
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Nil(t, found.FolderID)
	require.Equal(t, "Plan", found.Title)
}
