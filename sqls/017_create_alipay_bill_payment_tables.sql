CREATE TABLE IF NOT EXISTS payment_alipay_bill_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_order_no TEXT NOT NULL,
    account_log_id TEXT NOT NULL DEFAULT '',
    order_no TEXT NOT NULL DEFAULT '',
    amount_cents INTEGER NOT NULL,
    direction TEXT NOT NULL,
    remark TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    opposite_account TEXT NOT NULL DEFAULT '',
    paid_at DATETIME NOT NULL,
    raw TEXT NOT NULL DEFAULT '{}',
    matched_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_alipay_bill_records_provider_order_no ON payment_alipay_bill_records(provider_order_no);
CREATE INDEX IF NOT EXISTS idx_payment_alipay_bill_records_order_no ON payment_alipay_bill_records(order_no);
CREATE INDEX IF NOT EXISTS idx_payment_alipay_bill_records_paid_at ON payment_alipay_bill_records(paid_at);

CREATE TABLE IF NOT EXISTS payment_provider_states (
    provider TEXT NOT NULL,
    state_key TEXT NOT NULL,
    state_value TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (provider, state_key)
);
