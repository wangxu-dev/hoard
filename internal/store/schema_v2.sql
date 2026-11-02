CREATE TABLE folders (
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  remote_id TEXT NOT NULL,
  title TEXT NOT NULL,
  remote_count INTEGER NOT NULL,
  PRIMARY KEY(account_id,remote_id)
);
CREATE TABLE folder_works (
  account_id TEXT NOT NULL,
  folder_id TEXT NOT NULL,
  work_id TEXT NOT NULL REFERENCES works(id) ON DELETE CASCADE,
  favorited_at INTEGER NOT NULL,
  PRIMARY KEY(account_id,folder_id,work_id),
  FOREIGN KEY(account_id,folder_id) REFERENCES folders(account_id,remote_id) ON DELETE CASCADE
);
CREATE INDEX folder_works_work ON folder_works(work_id);
