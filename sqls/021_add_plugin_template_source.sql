ALTER TABLE plugin_template_metadata
    ADD COLUMN template_source TEXT NOT NULL DEFAULT '{}';
