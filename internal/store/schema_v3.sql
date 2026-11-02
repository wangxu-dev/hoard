CREATE TABLE work_account_state (
 account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
 work_id TEXT NOT NULL REFERENCES works(id) ON DELETE CASCADE,
 liked INTEGER CHECK(liked IN (0,1)),
 liked_at INTEGER,
 favorited INTEGER CHECK(favorited IN (0,1)),
 favorited_at INTEGER,
 coins INTEGER CHECK(coins BETWEEN 0 AND 2),
 coins_at INTEGER,
 PRIMARY KEY(account_id,work_id)
);
