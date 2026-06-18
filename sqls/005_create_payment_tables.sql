CREATE TABLE IF NOT EXISTS payment_orders (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  order_no TEXT NOT NULL UNIQUE,
  plugin_id TEXT NOT NULL DEFAULT '',
  union_id TEXT NOT NULL,
  platform TEXT NOT NULL DEFAULT '',
  adapter_id TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
  group_id TEXT NOT NULL DEFAULT '',
  subject TEXT NOT NULL,
  amount_cents INTEGER NOT NULL,
  points_amount INTEGER NOT NULL,
  provider TEXT NOT NULL,
  method TEXT NOT NULL,
  status TEXT NOT NULL,
  provider_order_no TEXT NOT NULL DEFAULT '',
  pay_url TEXT NOT NULL DEFAULT '',
  qrcode TEXT NOT NULL DEFAULT '',
  notify_raw TEXT NOT NULL DEFAULT '',
  metadata TEXT NOT NULL DEFAULT '{}',
  expired_at DATETIME NOT NULL,
  paid_at DATETIME,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_payment_orders_union_id ON payment_orders(union_id);
CREATE INDEX IF NOT EXISTS idx_payment_orders_status ON payment_orders(status);
CREATE INDEX IF NOT EXISTS idx_payment_orders_provider_order_no ON payment_orders(provider, provider_order_no);
CREATE INDEX IF NOT EXISTS idx_payment_orders_created_at ON payment_orders(created_at);

CREATE TABLE IF NOT EXISTS payment_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  order_no TEXT NOT NULL,
  event_type TEXT NOT NULL,
  message TEXT NOT NULL DEFAULT '',
  payload TEXT NOT NULL DEFAULT '{}',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_payment_events_order_no ON payment_events(order_no);
CREATE INDEX IF NOT EXISTS idx_payment_events_created_at ON payment_events(created_at);

CREATE TABLE IF NOT EXISTS point_transactions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  union_id TEXT NOT NULL,
  delta INTEGER NOT NULL,
  balance_after INTEGER NOT NULL,
  source TEXT NOT NULL,
  source_id TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_point_transactions_union_id ON point_transactions(union_id);
CREATE INDEX IF NOT EXISTS idx_point_transactions_source ON point_transactions(source, source_id);
CREATE INDEX IF NOT EXISTS idx_point_transactions_created_at ON point_transactions(created_at);
