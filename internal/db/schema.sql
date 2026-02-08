PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS imports (
  id INTEGER PRIMARY KEY,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  source TEXT NOT NULL,
  file_name TEXT NOT NULL,
  sha256 TEXT NOT NULL,
  rows_total INTEGER NOT NULL,
  rows_inserted INTEGER NOT NULL,
  rows_skipped INTEGER NOT NULL,
  notes TEXT
);

CREATE TABLE IF NOT EXISTS transactions (
  id INTEGER PRIMARY KEY,
  import_id INTEGER REFERENCES imports(id) ON DELETE SET NULL,

  txn_date TEXT NOT NULL,
  processed_on TEXT,

  amount_cents INTEGER NOT NULL,
  currency TEXT NOT NULL DEFAULT 'AUD',

  account TEXT,
  txn_type TEXT,
  details TEXT,
  category_raw TEXT,
  merchant_raw TEXT,

  merchant_norm TEXT,
  category_norm TEXT,
  notes TEXT,

  row_hash TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),

  UNIQUE(row_hash)
);

CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions(txn_date);
CREATE INDEX IF NOT EXISTS idx_transactions_category ON transactions(category_norm);
CREATE INDEX IF NOT EXISTS idx_transactions_merchant ON transactions(merchant_norm);
