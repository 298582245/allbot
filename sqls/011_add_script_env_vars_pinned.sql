ALTER TABLE script_env_vars ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_script_env_vars_pinned ON script_env_vars(pinned DESC, name ASC, id ASC);
