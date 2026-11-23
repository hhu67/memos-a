import { Button, Menu, Tooltip } from "@usememos/ui";
import {
  ChevronRightIcon,
  FolderIcon,
  FolderMinusIcon,
  ListIcon,
  MoreHorizontalIcon,
  PencilIcon,
  PlusIcon,
  Trash2Icon,
} from "lucide-react";
import { useMemo, useState } from "react";
import { useMemoFilterContext } from "@/contexts/MemoFilterContext";
import { useDeleteFolder, useFolders } from "@/hooks/useFolderQueries";
import { buildFolderTree, collectFolderSubtree, type FolderNode } from "@/lib/folders";
import { showErrorToast } from "@/lib/toast";
import { cn } from "@/lib/utils";
import type { Folder } from "@/types/proto/api/folder_service_pb";
import { useTranslate } from "@/utils/i18n";
import ConfirmDialog from "../ConfirmDialog";
import FolderDialog from "./FolderDialog";
import {
  SIDEBAR_ROW_BOX_CLASSES,
  SIDEBAR_ROW_CLASSES,
  SIDEBAR_ROW_LABEL_CLASSES,
  SIDEBAR_ROW_SLOT_BUTTON_CLASSES,
  SidebarRowIconSlot,
  sidebarRowStateAttributes,
  sidebarRowStateClasses,
} from "./SidebarRow";
import SidebarSection from "./SidebarSection";

const INDENT_STEP = 12;

interface DialogState {
  open: boolean;
  folder?: Folder;
  parent?: string;
}

interface FoldersSectionProps {
  onSelect?: () => void;
}

const FolderRow = ({
  node,
  depth,
  activeFolder,
  collapsed,
  onToggle,
  onSelect,
  onMenu,
}: {
  node: FolderNode;
  depth: number;
  activeFolder?: string;
  collapsed: ReadonlySet<string>;
  onToggle: (name: string) => void;
  onSelect: (folder: Folder) => void;
  onMenu: (action: "create" | "rename" | "delete", folder: Folder) => void;
}) => {
  const t = useTranslate();
  const { folder, children } = node;
  const hasChildren = children.length > 0;
  const open = hasChildren && !collapsed.has(folder.name);
  const active = activeFolder === folder.name;

  return (
    <div className="w-full min-w-0">
      <div
        role="treeitem"
        aria-level={depth + 1}
        aria-selected={active || undefined}
        aria-expanded={hasChildren ? open : undefined}
        {...sidebarRowStateAttributes(active ? "checked" : "idle")}
        className={cn(SIDEBAR_ROW_BOX_CLASSES, sidebarRowStateClasses(active ? "checked" : "idle"))}
        style={{ paddingInlineStart: 8 + depth * INDENT_STEP }}
      >
        {hasChildren ? (
          <button
            type="button"
            aria-label={`${open ? t("common.collapse") : t("common.expand")} ${folder.title}`}
            aria-expanded={open}
            className={SIDEBAR_ROW_SLOT_BUTTON_CLASSES}
            onClick={() => onToggle(folder.name)}
          >
            <ChevronRightIcon
              aria-hidden="true"
              className={cn(
                "mu-icon-sm me-auto text-mu-text-secondary/70 transition-transform",
                open && "rotate-90",
                !open && "rtl:rotate-180",
              )}
            />
          </button>
        ) : (
          <SidebarRowIconSlot icon={FolderIcon} />
        )}
        <button
          type="button"
          aria-label={folder.title}
          aria-pressed={active || undefined}
          className={SIDEBAR_ROW_LABEL_CLASSES}
          onClick={() => onSelect(folder)}
        >
          {hasChildren && <SidebarRowIconSlot icon={FolderIcon} />}
          <span className="min-w-0 flex-1 truncate text-start">{folder.title}</span>
        </button>
        <Menu.Root modal={false}>
          <Tooltip.Root>
            <Tooltip.Trigger
              render={
                <Menu.Trigger
                  aria-label={`${folder.title}: ${t("common.more")}`}
                  render={<Button variant="ghost" size="xs" icon={<MoreHorizontalIcon />} />}
                />
              }
            />
            <Tooltip.Popup side="top">{t("common.more")}</Tooltip.Popup>
          </Tooltip.Root>
          <Menu.Popup align="end">
            <Menu.Group>
              <Menu.Item icon={<PlusIcon />} onClick={() => onMenu("create", folder)}>
                {t("folder.new-subfolder")}
              </Menu.Item>
              <Menu.Item icon={<PencilIcon />} onClick={() => onMenu("rename", folder)}>
                {t("folder.rename")}
              </Menu.Item>
            </Menu.Group>
            <Menu.Separator />
            <Menu.Item icon={<Trash2Icon />} className="text-mu-danger" onClick={() => onMenu("delete", folder)}>
              {t("common.delete")}
            </Menu.Item>
          </Menu.Popup>
        </Menu.Root>
      </div>
      {open && (
        <div className="mt-0.5 flex w-full min-w-0 flex-col gap-0.5" role="group">
          {children.map((child) => (
            <FolderRow
              key={child.folder.name}
              node={child}
              depth={depth + 1}
              activeFolder={activeFolder}
              collapsed={collapsed}
              onToggle={onToggle}
              onSelect={onSelect}
              onMenu={onMenu}
            />
          ))}
        </div>
      )}
    </div>
  );
};

const FoldersSection = ({ onSelect }: FoldersSectionProps) => {
  const t = useTranslate();
  const { data: folders = [] } = useFolders();
  const deleteFolder = useDeleteFolder();
  const { getFiltersByFactor, removeFiltersByFactor, addFilter } = useMemoFilterContext();
  const [collapsed, setCollapsed] = useState<Set<string>>(() => new Set());
  const [dialog, setDialog] = useState<DialogState>({ open: false });
  const [deleting, setDeleting] = useState<Folder | undefined>();

  const tree = useMemo(() => buildFolderTree(folders), [folders]);
  const activeFolder = getFiltersByFactor?.("folder")[0]?.value;
  const noFolder = (getFiltersByFactor?.("noFolder").length ?? 0) > 0;
  const allActive = !activeFolder && !noFolder;

  const selectAll = () => {
    removeFiltersByFactor?.("folder");
    removeFiltersByFactor?.("noFolder");
    onSelect?.();
  };

  const selectNoFolder = () => {
    removeFiltersByFactor?.("folder");
    removeFiltersByFactor?.("noFolder");
    addFilter?.({ factor: "noFolder", value: "" });
    onSelect?.();
  };

  const selectFolder = (folder: Folder) => {
    removeFiltersByFactor?.("folder");
    removeFiltersByFactor?.("noFolder");
    addFilter?.({ factor: "folder", value: folder.name });
    onSelect?.();
  };

  const toggle = (name: string) => {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  };

  const handleMenu = (action: "create" | "rename" | "delete", folder: Folder) => {
    if (action === "delete") {
      setDeleting(folder);
      return;
    }
    setDialog({ open: true, folder: action === "rename" ? folder : undefined, parent: action === "create" ? folder.name : undefined });
  };

  // A folder being deleted takes its descendants with it; detaching them first
  // keeps the tree from showing rows that are already gone.
  const handleDelete = async () => {
    if (!deleting) return;
    try {
      await deleteFolder.mutateAsync(deleting.name);
      const removed = collectFolderSubtree(folders, deleting.name);
      if (activeFolder && removed.has(activeFolder)) removeFiltersByFactor?.("folder");
    } catch (error) {
      console.error(error);
      showErrorToast(t("folder.delete-error"));
    }
  };

  return (
    <SidebarSection
      label={t("common.folders")}
      action={
        <Tooltip.Root>
          <Tooltip.Trigger
            render={
              <Button
                variant="ghost"
                size="xs"
                icon={<PlusIcon />}
                aria-label={t("folder.new")}
                onClick={() => setDialog({ open: true })}
              />
            }
          />
          <Tooltip.Popup side="top">{t("folder.new")}</Tooltip.Popup>
        </Tooltip.Root>
      }
    >
      <button
        type="button"
        aria-pressed={allActive || undefined}
        {...sidebarRowStateAttributes(allActive ? "checked" : "idle")}
        className={cn(SIDEBAR_ROW_CLASSES, sidebarRowStateClasses(allActive ? "checked" : "idle"))}
        onClick={selectAll}
      >
        <SidebarRowIconSlot icon={ListIcon} />
        <span className="min-w-0 flex-1 truncate text-start">{t("folder.all")}</span>
      </button>
      <button
        type="button"
        aria-pressed={noFolder || undefined}
        {...sidebarRowStateAttributes(noFolder ? "checked" : "idle")}
        className={cn(SIDEBAR_ROW_CLASSES, sidebarRowStateClasses(noFolder ? "checked" : "idle"))}
        onClick={selectNoFolder}
      >
        <SidebarRowIconSlot icon={FolderMinusIcon} />
        <span className="min-w-0 flex-1 truncate text-start">{t("folder.none")}</span>
      </button>
      {tree.map((node) => (
        <FolderRow
          key={node.folder.name}
          node={node}
          depth={0}
          activeFolder={activeFolder}
          collapsed={collapsed}
          onToggle={toggle}
          onSelect={selectFolder}
          onMenu={handleMenu}
        />
      ))}

      <FolderDialog
        open={dialog.open}
        onOpenChange={(open) => setDialog((prev) => ({ ...prev, open }))}
        folder={dialog.folder}
        parent={dialog.parent}
      />
      <ConfirmDialog
        open={Boolean(deleting)}
        onOpenChange={(open) => !open && setDeleting(undefined)}
        title={t("folder.delete-title")}
        description={t("folder.delete-description", { name: deleting?.title ?? "" })}
        confirmLabel={t("common.delete")}
        cancelLabel={t("common.cancel")}
        confirmVariant="danger"
        onConfirm={handleDelete}
      />
    </SidebarSection>
  );
};

export default FoldersSection;
