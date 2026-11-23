import { Button, Dialog, Field, Input } from "@usememos/ui";
import { type FormEvent, useEffect, useState } from "react";
import { useCreateFolder, useUpdateFolder } from "@/hooks/useFolderQueries";
import { showErrorToast, toast } from "@/lib/toast";
import type { Folder } from "@/types/proto/api/folder_service_pb";
import { useTranslate } from "@/utils/i18n";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The folder to rename; omit to create a new folder. */
  folder?: Folder;
  /** The parent of a new subfolder, as a resource name. */
  parent?: string;
}

const FolderDialog = ({ open, onOpenChange, folder, parent }: Props) => {
  const t = useTranslate();
  const createFolder = useCreateFolder();
  const updateFolder = useUpdateFolder();
  const [title, setTitle] = useState("");
  const isPending = createFolder.isPending || updateFolder.isPending;

  useEffect(() => {
    if (open) setTitle(folder?.title ?? "");
  }, [open, folder]);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmed = title.trim();
    if (!trimmed || isPending) return;
    try {
      if (folder) {
        await updateFolder.mutateAsync({ name: folder.name, title: trimmed });
      } else {
        await createFolder.mutateAsync({ title: trimmed, parent });
      }
      toast.add({ title: folder ? t("folder.rename-success") : t("folder.create-success"), type: "success" });
      onOpenChange(false);
    } catch (error) {
      console.error(error);
      showErrorToast(t("folder.save-error"));
    }
  };

  return (
    <Dialog.Root open={open} onOpenChange={(next) => !isPending && onOpenChange(next)}>
      <Dialog.Popup closeLabel={t("common.close")}>
        <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
          <Dialog.Title>{folder ? t("folder.rename") : parent ? t("folder.new-subfolder") : t("folder.new")}</Dialog.Title>
          <Field.Root>
            <Field.Label>{t("folder.name")}</Field.Label>
            <Input
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              placeholder={t("folder.name-placeholder")}
              autoComplete="off"
              autoFocus
            />
          </Field.Root>
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" disabled={isPending} onClick={() => onOpenChange(false)}>
              {t("common.cancel")}
            </Button>
            <Button type="submit" variant="primary" disabled={!title.trim()} loading={isPending} loadingLabel={t("common.save")}>
              {folder ? t("common.save") : t("common.create")}
            </Button>
          </div>
        </form>
      </Dialog.Popup>
    </Dialog.Root>
  );
};

export default FolderDialog;
