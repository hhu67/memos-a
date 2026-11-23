CREATE TABLE folder (
  id SERIAL PRIMARY KEY,
  uid TEXT NOT NULL UNIQUE,
  creator_id INTEGER NOT NULL,
  parent_id INTEGER DEFAULT NULL,
  name TEXT NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  updated_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())
);

CREATE INDEX idx_folder_creator_id ON folder(creator_id, id);
CREATE INDEX idx_folder_parent_id ON folder(parent_id);

ALTER TABLE memo ADD COLUMN title TEXT NOT NULL DEFAULT '';
ALTER TABLE memo ADD COLUMN folder_id INTEGER DEFAULT NULL;

CREATE INDEX idx_memo_folder_id ON memo(folder_id, row_status, created_ts DESC, id DESC);
