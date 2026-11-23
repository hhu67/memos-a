import { useTranslate } from "@/utils/i18n";
import { useEditorContext, useEditorSelector } from "../state";

/**
 * The optional title above the content. It stays a quiet line in the composer —
 * no box or label — and only shows its reserved focus border when reached.
 */
export const EditorTitle = () => {
  const t = useTranslate();
  const { actions, dispatch } = useEditorContext();
  const title = useEditorSelector((s) => s.title);
  const isSaving = useEditorSelector((s) => s.ui.isLoading.saving);

  return (
    <input
      value={title}
      onChange={(event) => dispatch(actions.setTitle(event.target.value))}
      placeholder={t("editor.title-placeholder")}
      aria-label={t("common.title")}
      maxLength={512}
      disabled={isSaving}
      autoComplete="off"
      className="w-full min-w-0 border border-transparent bg-transparent px-0 py-0.5 mu-title text-mu-text placeholder:text-mu-text-secondary/60 mu-focus focus-visible:border-mu-focus"
    />
  );
};
