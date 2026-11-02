CREATE TABLE accounts (
  id            TEXT PRIMARY KEY,
  source        TEXT NOT NULL,
  remote_id     TEXT,
  nickname      TEXT NOT NULL DEFAULT '',
  avatar_url    TEXT,
  avatar_hash   TEXT REFERENCES blobs(hash),
  session_name  TEXT NOT NULL UNIQUE,
  status        TEXT NOT NULL,
  position      INTEGER NOT NULL,
  created_at    INTEGER NOT NULL,
  last_used_at  INTEGER NOT NULL,
  UNIQUE (source, remote_id)
);


CREATE TABLE authors (
  id           TEXT PRIMARY KEY,
  source       TEXT NOT NULL,
  remote_id    TEXT NOT NULL,
  nickname     TEXT NOT NULL,
  avatar_url   TEXT,
  avatar_hash  TEXT REFERENCES blobs(hash),
  updated_at   INTEGER NOT NULL
);

CREATE TABLE works (
  id           TEXT PRIMARY KEY,
  source       TEXT NOT NULL,
  remote_id    TEXT NOT NULL,
  kind         TEXT NOT NULL,
  title        TEXT NOT NULL,
  author_id    TEXT REFERENCES authors(id),
  published_at INTEGER NOT NULL,
  duration_ms  INTEGER NOT NULL DEFAULT 0,
  width        INTEGER NOT NULL DEFAULT 0,
  height       INTEGER NOT NULL DEFAULT 0,
  image_count  INTEGER NOT NULL DEFAULT 0,
  is_live      INTEGER NOT NULL DEFAULT 0,
  cover_url    TEXT,
  cover_hash   TEXT REFERENCES blobs(hash),
  available    INTEGER NOT NULL DEFAULT 1,
  extra        TEXT NOT NULL DEFAULT '{}',
  updated_at   INTEGER NOT NULL
);
CREATE INDEX works_published ON works(published_at DESC, id);
CREATE INDEX works_author ON works(author_id);


CREATE TABLE relations (
  account_id  TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  work_id     TEXT NOT NULL REFERENCES works(id),
  kind        TEXT NOT NULL,
  seq         INTEGER NOT NULL,
  published_at INTEGER NOT NULL,
  first_seen  INTEGER NOT NULL,
  pending     TEXT,
  PRIMARY KEY (account_id, kind, work_id)
);
CREATE INDEX relations_list ON relations(account_id, kind, seq DESC);
CREATE INDEX relations_time ON relations(account_id, kind, published_at DESC, work_id);
CREATE INDEX relations_work ON relations(work_id);

CREATE TABLE musics (
  id          TEXT PRIMARY KEY,
  source      TEXT NOT NULL,
  remote_id   TEXT NOT NULL,
  title       TEXT NOT NULL,
  author      TEXT NOT NULL,
  duration_ms INTEGER NOT NULL DEFAULT 0,
  cover_url   TEXT,
  cover_hash  TEXT REFERENCES blobs(hash),
  extra       TEXT NOT NULL DEFAULT '{}',
  updated_at  INTEGER NOT NULL
);
CREATE TABLE music_relations (
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  music_id   TEXT NOT NULL REFERENCES musics(id),
  seq        INTEGER NOT NULL,
  first_seen INTEGER NOT NULL,
  PRIMARY KEY (account_id, music_id)
);

CREATE TABLE sync_state (
  account_id      TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  kind            TEXT NOT NULL,
  phase           TEXT NOT NULL,
  status          TEXT NOT NULL,
  head_cursor     TEXT NOT NULL DEFAULT '0',
  backfill_cursor TEXT,
  head_seq        INTEGER NOT NULL DEFAULT 0,
  pages           INTEGER NOT NULL DEFAULT 0,
  items           INTEGER NOT NULL DEFAULT 0,
  remote_total    INTEGER,
  error_code      TEXT,
  retry_at        INTEGER,
  failures        INTEGER NOT NULL DEFAULT 0,
  last_full_at    INTEGER,
  updated_at      INTEGER NOT NULL,
  PRIMARY KEY (account_id, kind)
);

CREATE TABLE blobs (
  hash         TEXT PRIMARY KEY,
  path         TEXT NOT NULL,
  bytes        INTEGER NOT NULL,
  mime         TEXT NOT NULL,
  created_at   INTEGER NOT NULL,
  last_used_at INTEGER NOT NULL
);

CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
