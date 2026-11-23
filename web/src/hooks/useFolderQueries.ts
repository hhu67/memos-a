import { create } from "@bufbuild/protobuf";
import { FieldMaskSchema } from "@bufbuild/protobuf/wkt";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { folderServiceClient } from "@/connect";
import { memoKeys } from "@/hooks/useMemoQueries";
import { FolderSchema, ListFoldersRequestSchema, UpdateFolderRequestSchema } from "@/types/proto/api/folder_service_pb";

export const folderKeys = {
  all: ["folders"] as const,
  list: () => [...folderKeys.all, "list"] as const,
};

/** Every folder the signed-in user owns. Folder trees are small, so one page holds them all. */
export function useFolders(options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: folderKeys.list(),
    queryFn: async () => {
      const response = await folderServiceClient.listFolders(create(ListFoldersRequestSchema, { pageSize: 1000 }));
      return response.folders;
    },
    staleTime: 1000 * 60,
    enabled: options?.enabled ?? true,
  });
}

export function useCreateFolder() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ title, parent }: { title: string; parent?: string }) => {
      return folderServiceClient.createFolder({
        folder: create(FolderSchema, { title, parent }),
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: folderKeys.all });
    },
  });
}

export function useUpdateFolder() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ name, title, parent }: { name: string; title?: string; parent?: string | null }) => {
      const updateMask: string[] = [];
      if (title !== undefined) updateMask.push("title");
      if (parent !== undefined) updateMask.push("parent");
      return folderServiceClient.updateFolder(
        create(UpdateFolderRequestSchema, {
          folder: create(FolderSchema, { name, title, parent: parent ?? undefined }),
          updateMask: create(FieldMaskSchema, { paths: updateMask }),
        }),
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: folderKeys.all });
      queryClient.invalidateQueries({ queryKey: memoKeys.lists() });
    },
  });
}

export function useDeleteFolder() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (name: string) => {
      await folderServiceClient.deleteFolder({ name });
      return name;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: folderKeys.all });
      // Deleting a folder detaches its memos, so their lists change too.
      queryClient.invalidateQueries({ queryKey: memoKeys.lists() });
    },
  });
}
