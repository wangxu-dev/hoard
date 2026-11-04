ALTER TABLE sync_state ADD COLUMN delay_ms INTEGER NOT NULL DEFAULT 0;

CREATE TABLE sync_checkpoints (
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  scope TEXT NOT NULL,
  next_page INTEGER NOT NULL,
  remote_count INTEGER NOT NULL,
  PRIMARY KEY(account_id, scope)
);
