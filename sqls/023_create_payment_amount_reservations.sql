CREATE TABLE IF NOT EXISTS payment_amount_reservations (
    provider TEXT NOT NULL,
    amount_cents INTEGER NOT NULL,
    order_no TEXT NOT NULL UNIQUE,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (provider, amount_cents)
);

CREATE INDEX IF NOT EXISTS idx_payment_amount_reservations_expires_at
    ON payment_amount_reservations(expires_at);

INSERT OR IGNORE INTO payment_amount_reservations (
    provider,
    amount_cents,
    order_no,
    expires_at,
    created_at
)
SELECT
    provider,
    amount_cents,
    order_no,
    expired_at,
    CURRENT_TIMESTAMP
FROM payment_orders
WHERE provider = 'alipay_bill'
  AND status = 'pending'
  AND expired_at > CURRENT_TIMESTAMP;
