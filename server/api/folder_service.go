package api

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	apipb "github.com/usememos/memos/proto/gen/api"
	"github.com/usememos/memos/store"
)

func buildFolderName(uid string) string {
	return FolderNamePrefix + uid
}

func (s *APIService) requireFolderUser(ctx context.Context) (*store.User, error) {
	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return nil, status.Error(codes.Unauthenticated, "user not authenticated")
	}
	return user, nil
}

func mapFolderMutationError(err error, operation string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, store.ErrFolderNotFound):
		return status.Error(codes.NotFound, "folder not found")
	case errors.Is(err, store.ErrFolderPermissionDenied):
		return status.Error(codes.PermissionDenied, "permission denied")
	case errors.Is(err, store.ErrFolderAlreadyExists):
		return status.Error(codes.AlreadyExists, "folder already exists")
	case errors.Is(err, sql.ErrNoRows):
		return status.Error(codes.NotFound, "folder not found")
	default:
		return status.Errorf(codes.Internal, "%s: %v", operation, err)
	}
}

// listUserFolders loads every folder the user owns, keyed by ID. Folder trees
// are small and personal, so one read serves validation and parent resolution.
func (s *APIService) listUserFolders(ctx context.Context, userID int32) ([]*store.Folder, map[int32]*store.Folder, map[string]*store.Folder, error) {
	folders, err := s.Store.ListFolders(ctx, &store.FindFolder{CreatorID: &userID})
	if err != nil {
		return nil, nil, nil, err
	}
	byID := make(map[int32]*store.Folder, len(folders))
	byUID := make(map[string]*store.Folder, len(folders))
	for _, folder := range folders {
		byID[folder.ID] = folder
		byUID[folder.UID] = folder
	}
	return folders, byID, byUID, nil
}

// resolveOwnedFolder returns the caller's folder named by a resource name.
func (s *APIService) resolveOwnedFolder(ctx context.Context, name string, userID int32) (*store.Folder, error) {
	uid, err := ExtractFolderUIDFromName(name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid folder name: %v", err)
	}
	folder, err := s.Store.GetFolder(ctx, &store.FindFolder{UID: &uid, CreatorID: &userID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get folder: %v", err)
	}
	if folder == nil {
		return nil, status.Error(codes.NotFound, "folder not found")
	}
	return folder, nil
}

// resolveFolderParent validates a parent resource name and rejects a move that
// would make the folder its own ancestor.
func (s *APIService) resolveFolderParent(ctx context.Context, name string, userID, movingFolderID int32, byID map[int32]*store.Folder, byUID map[string]*store.Folder) (*store.Folder, error) {
	if strings.TrimSpace(name) == "" {
		return nil, nil
	}
	uid, err := ExtractFolderUIDFromName(name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid parent folder name: %v", err)
	}
	parent := byUID[uid]
	if parent == nil {
		return nil, status.Error(codes.NotFound, "parent folder not found")
	}
	if movingFolderID != 0 {
		visited := make(map[int32]struct{}, len(byID))
		for cursor := parent; cursor != nil; {
			if _, seen := visited[cursor.ID]; seen {
				return nil, status.Error(codes.InvalidArgument, "folder tree contains a cycle")
			}
			visited[cursor.ID] = struct{}{}
			if cursor.ID == movingFolderID {
				return nil, status.Error(codes.InvalidArgument, "a folder cannot be moved under itself")
			}
			if cursor.ParentID == nil {
				break
			}
			cursor = byID[*cursor.ParentID]
		}
	}
	return parent, nil
}

func (s *APIService) convertFolderFromStore(folder *store.Folder, byID map[int32]*store.Folder) *apipb.Folder {
	if folder == nil {
		return nil
	}
	converted := &apipb.Folder{
		Name:       buildFolderName(folder.UID),
		Title:      folder.Name,
		CreateTime: timestamppb.New(time.Unix(folder.CreatedTs, 0)),
		UpdateTime: timestamppb.New(time.Unix(folder.UpdatedTs, 0)),
	}
	if folder.ParentID != nil {
		if parent := byID[*folder.ParentID]; parent != nil {
			parentName := buildFolderName(parent.UID)
			converted.Parent = &parentName
		}
	}
	return converted
}

// CreateFolder creates a folder owned by the authenticated caller.
func (s *APIService) CreateFolder(ctx context.Context, request *apipb.CreateFolderRequest) (*apipb.Folder, error) {
	user, err := s.requireFolderUser(ctx)
	if err != nil {
		return nil, err
	}
	if request.GetFolder() == nil {
		return nil, status.Error(codes.InvalidArgument, "folder is required")
	}
	title := strings.TrimSpace(request.Folder.Title)
	if title == "" {
		return nil, status.Error(codes.InvalidArgument, "folder title is required")
	}
	uid, err := ValidateAndGenerateUID(request.FolderId)
	if err != nil {
		return nil, err
	}
	_, byID, byUID, err := s.listUserFolders(ctx, user.ID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list folders: %v", err)
	}
	parent, err := s.resolveFolderParent(ctx, request.Folder.GetParent(), user.ID, 0, byID, byUID)
	if err != nil {
		return nil, err
	}
	create := &store.Folder{
		UID:       uid,
		CreatorID: user.ID,
		Name:      title,
	}
	if parent != nil {
		create.ParentID = &parent.ID
	}
	created, err := s.Store.CreateFolder(ctx, create)
	if err != nil {
		return nil, mapFolderMutationError(err, "failed to create folder")
	}
	byID[created.ID] = created
	s.SSEHub.publishMemoChanged()
	return s.convertFolderFromStore(created, byID), nil
}

// ListFolders lists the authenticated caller's folders.
func (s *APIService) ListFolders(ctx context.Context, request *apipb.ListFoldersRequest) (*apipb.ListFoldersResponse, error) {
	user, err := s.requireFolderUser(ctx)
	if err != nil {
		return nil, err
	}
	limit, offset, err := listSpacePage(request.PageSize, request.PageToken)
	if err != nil {
		return nil, err
	}
	limitPlusOne := limit + 1
	folders, err := s.Store.ListFolders(ctx, &store.FindFolder{
		CreatorID: &user.ID,
		Limit:     &limitPlusOne,
		Offset:    &offset,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list folders: %v", err)
	}
	nextPageToken := ""
	if len(folders) == limitPlusOne {
		folders = folders[:limit]
		nextPageToken, err = getPageToken(limit, offset+limit)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to create next page token: %v", err)
		}
	}
	// Every page needs the whole tree to resolve parents, so read all of the
	// caller's folders for the lookup even when the page is smaller.
	_, byID, _, err := s.listUserFolders(ctx, user.ID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list folders: %v", err)
	}
	response := &apipb.ListFoldersResponse{Folders: make([]*apipb.Folder, 0, len(folders)), NextPageToken: nextPageToken}
	for _, folder := range folders {
		response.Folders = append(response.Folders, s.convertFolderFromStore(folder, byID))
	}
	return response, nil
}

// GetFolder gets one of the authenticated caller's folders.
func (s *APIService) GetFolder(ctx context.Context, request *apipb.GetFolderRequest) (*apipb.Folder, error) {
	user, err := s.requireFolderUser(ctx)
	if err != nil {
		return nil, err
	}
	folder, err := s.resolveOwnedFolder(ctx, request.Name, user.ID)
	if err != nil {
		return nil, err
	}
	_, byID, _, err := s.listUserFolders(ctx, user.ID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list folders: %v", err)
	}
	return s.convertFolderFromStore(folder, byID), nil
}

// UpdateFolder renames or moves one of the authenticated caller's folders.
func (s *APIService) UpdateFolder(ctx context.Context, request *apipb.UpdateFolderRequest) (*apipb.Folder, error) {
	user, err := s.requireFolderUser(ctx)
	if err != nil {
		return nil, err
	}
	if request.GetFolder() == nil {
		return nil, status.Error(codes.InvalidArgument, "folder is required")
	}
	if request.UpdateMask == nil || len(request.UpdateMask.Paths) == 0 {
		return nil, status.Error(codes.InvalidArgument, "update mask is required")
	}
	folder, err := s.resolveOwnedFolder(ctx, request.Folder.Name, user.ID)
	if err != nil {
		return nil, err
	}
	_, byID, byUID, err := s.listUserFolders(ctx, user.ID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list folders: %v", err)
	}
	update := &store.UpdateFolder{ID: folder.ID, ActorUserID: user.ID}
	for _, path := range request.UpdateMask.Paths {
		switch path {
		case "title":
			title := strings.TrimSpace(request.Folder.Title)
			if title == "" {
				return nil, status.Error(codes.InvalidArgument, "folder title is required")
			}
			update.Name = &title
		case "parent":
			parent, err := s.resolveFolderParent(ctx, request.Folder.GetParent(), user.ID, folder.ID, byID, byUID)
			if err != nil {
				return nil, err
			}
			if parent == nil {
				update.ClearParent = true
			} else {
				update.ParentID = &parent.ID
			}
		default:
			return nil, status.Errorf(codes.InvalidArgument, "invalid update path: %s", path)
		}
	}
	updated, err := s.Store.UpdateFolder(ctx, update)
	if err != nil {
		return nil, mapFolderMutationError(err, "failed to update folder")
	}
	byID[updated.ID] = updated
	s.SSEHub.publishMemoChanged()
	return s.convertFolderFromStore(updated, byID), nil
}

// DeleteFolder deletes a folder subtree and detaches its memos.
func (s *APIService) DeleteFolder(ctx context.Context, request *apipb.DeleteFolderRequest) (*emptypb.Empty, error) {
	user, err := s.requireFolderUser(ctx)
	if err != nil {
		return nil, err
	}
	folder, err := s.resolveOwnedFolder(ctx, request.Name, user.ID)
	if err != nil {
		return nil, err
	}
	if err := s.Store.DeleteFolder(ctx, &store.DeleteFolder{ID: folder.ID, ActorUserID: user.ID}); err != nil {
		return nil, mapFolderMutationError(err, "failed to delete folder")
	}
	s.SSEHub.publishMemoChanged()
	return &emptypb.Empty{}, nil
}
