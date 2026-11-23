import type { Folder } from "@/types/proto/api/folder_service_pb";

export const FOLDER_NAME_PREFIX = "folders/";

export const getFolderUid = (name: string): string => name.slice(FOLDER_NAME_PREFIX.length);

export const buildFolderName = (uid: string): string => `${FOLDER_NAME_PREFIX}${uid}`;

export interface FolderNode {
  folder: Folder;
  children: FolderNode[];
}

/** Builds a title-sorted tree from the flat folder list the server returns. */
export const buildFolderTree = (folders: Folder[]): FolderNode[] => {
  const nodes = new Map<string, FolderNode>();
  for (const folder of folders) {
    nodes.set(folder.name, { folder, children: [] });
  }
  const roots: FolderNode[] = [];
  for (const node of nodes.values()) {
    const parent = node.folder.parent ? nodes.get(node.folder.parent) : undefined;
    if (parent && parent !== node) {
      parent.children.push(node);
    } else {
      roots.push(node);
    }
  }
  const sort = (list: FolderNode[]) => {
    list.sort((a, b) => a.folder.title.localeCompare(b.folder.title));
    list.forEach((node) => sort(node.children));
  };
  sort(roots);
  return roots;
};

/** Every folder name below `name`, the folder itself included. */
export const collectFolderSubtree = (folders: Folder[], name: string): Set<string> => {
  const result = new Set<string>([name]);
  let changed = true;
  while (changed) {
    changed = false;
    for (const folder of folders) {
      if (folder.parent && result.has(folder.parent) && !result.has(folder.name)) {
        result.add(folder.name);
        changed = true;
      }
    }
  }
  return result;
};
