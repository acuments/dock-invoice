-- Schema for the invoicer SQLite database. Applied idempotently on every
-- Open() via "CREATE TABLE IF NOT EXISTS" statements.

CREATE TABLE IF NOT EXISTS settings (
    id   INTEGER PRIMARY KEY CHECK (id = 1),
    data TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS customers (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    name              TEXT NOT NULL,
    gstin             TEXT NOT NULL DEFAULT '',
    billing_address   TEXT NOT NULL DEFAULT '[]',
    shipping_address  TEXT NOT NULL DEFAULT '[]'
);

-- currency is new: a saved item's default rate (rate_usd — kept as the
-- column name for compatibility with pre-existing databases, but no longer
-- assumed to be USD) can now be priced in INR or USD (see
-- model.Item.Currency). New databases get the column straight from this
-- CREATE TABLE; existing ones get it via the ALTER TABLE migration in
-- db.go (CREATE TABLE IF NOT EXISTS is a no-op once the table already
-- exists, so this statement alone never touches pre-existing rows).
CREATE TABLE IF NOT EXISTS items (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    description TEXT NOT NULL,
    hsn_sac     TEXT NOT NULL DEFAULT '',
    unit        TEXT NOT NULL DEFAULT '',
    rate_usd    INTEGER NOT NULL DEFAULT 0,
    currency    TEXT NOT NULL DEFAULT 'USD'
);

-- Invoices are stored as a JSON snapshot (data) of the full model.Invoice —
-- including a copy of the customer and seller/bank details at save time —
-- so later edits to the customers/settings tables never alter a
-- previously-saved invoice. A handful of columns are duplicated out of the
-- JSON purely to make listing/filtering cheap.
CREATE TABLE IF NOT EXISTS invoices (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    number        TEXT NOT NULL UNIQUE,
    type          TEXT NOT NULL,
    customer_name TEXT NOT NULL DEFAULT '',
    invoice_date  TEXT NOT NULL,
    data          TEXT NOT NULL,
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

-- One row per numbering series (pattern + financial year), used to
-- atomically allocate the next sequence number.
CREATE TABLE IF NOT EXISTS number_series (
    series_key TEXT PRIMARY KEY,
    last_seq   INTEGER NOT NULL DEFAULT 0
);
