import { Badge } from "@usememos/ui";
import { FolderIcon } from "lucide-react";
import { Link } from "react-router-dom";
import { getFilterSearch } from "@/contexts/MemoFilterContext";
import useCurrentUser from "@/hooks/useCurrentUser";
import { useFolders } from "@/hooks/useFolderQueries";
import { cn } from "@/lib/utils";
import { useTranslate } from "@/utils/i18n";

interface MemoFolderBadgeProps {
  folderName?: string;
}

/** The memo's folder as a badge; clicking it opens that folder's feed. */
const MemoFolderBadge = ({ folderName }: MemoFolderBadgeProps) => {
  const t = useTranslate();
  const currentUser = useCurrentUser();
  const { data: folders = [] } = useFolders({ enabled: Boolean(currentUser) && Boolean(folderName) });

  if (!folderName) return null;
  const folder = folders.find((item) => item.name === folderName);
  if (!folder) return null;

  return (
    <Link
      to={`/${getFilterSearch([{ factor: "folder", value: folder.name }])}`}
      title={`${t("common.folders")}: ${folder.title}`}
      aria-label={`${t("common.folders")}: ${folder.title}`}
      className={cn("min-w-0 shrink max-w-48 rounded-mu-sm border border-transparent mu-focus focus-visible:border-mu-focus")}
    >
      <Badge tone="outline" icon={<FolderIcon className="mu-icon-xs" />}>
        <span className="min-w-0 truncate">{folder.title}</span>
      </Badge>
    </Link>
  );
};

export default MemoFolderBadge;
