import { buttonClasses, Menu } from "@usememos/ui";
import { ChevronDownIcon, FolderIcon } from "lucide-react";
import { useMemo } from "react";
import { useFolders } from "@/hooks/useFolderQueries";
import { buildFolderTree, type FolderNode } from "@/lib/folders";
import { cn } from "@/lib/utils";
import { useTranslate } from "@/utils/i18n";

const NO_FOLDER = "__none__";

const flatten = (nodes: FolderNode[], depth = 0): Array<{ name: string; title: string; depth: number }> =>
  nodes.flatMap((node) => [{ name: node.folder.name, title: node.folder.title, depth }, ...flatten(node.children, depth + 1)]);

/** The composer's folder picker: a quiet toolbar control, one choice per folder. */
const FolderMenu = ({ value, onChange, disabled }: { value?: string; onChange: (folder?: string) => void; disabled?: boolean }) => {
  const t = useTranslate();
  const { data: folders = [], isPending, isError, refetch } = useFolders();
  const choices = useMemo(() => flatten(buildFolderTree(folders)), [folders]);
  const selected = choices.find((choice) => choice.name === value);
  const summary = selected?.title ?? t("editor.folder");

  return (
    <Menu.Root modal={false}>
      <Menu.Trigger
        className={cn(buttonClasses({ variant: "ghost", size: "md" }), "min-w-0 max-w-48")}
        disabled={disabled}
        title={summary}
        aria-label={summary}
      >
        <FolderIcon className="mu-icon" />
        <span className="truncate">{summary}</span>
        <ChevronDownIcon className="mu-icon-sm" />
      </Menu.Trigger>
      <Menu.Popup align="start" className="w-56" aria-label={t("common.folders")}>
        <Menu.RadioGroup value={value ?? NO_FOLDER} onValueChange={(next) => onChange(next === NO_FOLDER ? undefined : next)}>
          <Menu.RadioItem value={NO_FOLDER} closeOnClick icon={<FolderIcon />}>
            {t("folder.none")}
          </Menu.RadioItem>
          {choices.map((choice) => (
            <Menu.RadioItem key={choice.name} value={choice.name} closeOnClick icon={<FolderIcon />}>
              <span className="truncate" style={{ paddingInlineStart: choice.depth * 10 }}>
                {choice.title}
              </span>
            </Menu.RadioItem>
          ))}
          {isPending && <Menu.Item disabled>{t("folder.loading")}</Menu.Item>}
          {isError && (
            <Menu.Item closeOnClick={false} onClick={() => void refetch()} shortcut={t("search.retry")}>
              <span className="text-mu-danger">{t("folder.load-error")}</span>
            </Menu.Item>
          )}
        </Menu.RadioGroup>
        {choices.length === 0 && !isPending && !isError && <Menu.Item disabled>{t("folder.empty")}</Menu.Item>}
      </Menu.Popup>
    </Menu.Root>
  );
};

export default FolderMenu;
