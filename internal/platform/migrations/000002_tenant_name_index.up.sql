CREATE UNIQUE INDEX IF NOT EXISTS idx_tenants_display_name_no_installation
    ON tenants (display_name) WHERE github_installation_id IS NULL;
