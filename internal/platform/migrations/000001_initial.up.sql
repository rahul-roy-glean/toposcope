CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name TEXT NOT NULL,
    github_installation_id BIGINT UNIQUE,
    credentials_ref TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE repositories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    github_repo_id BIGINT,
    full_name TEXT NOT NULL,
    default_branch TEXT NOT NULL DEFAULT 'main',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, full_name)
);

CREATE TABLE snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    repo_id UUID NOT NULL REFERENCES repositories(id),
    commit_sha TEXT NOT NULL,
    branch TEXT,
    node_count INTEGER NOT NULL,
    edge_count INTEGER NOT NULL,
    package_count INTEGER NOT NULL,
    extraction_ms INTEGER NOT NULL,
    storage_ref TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(repo_id, commit_sha)
);

CREATE TABLE baselines (
    repo_id UUID PRIMARY KEY REFERENCES repositories(id),
    snapshot_id UUID NOT NULL REFERENCES snapshots(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE deltas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    repo_id UUID NOT NULL REFERENCES repositories(id),
    base_snapshot_id UUID NOT NULL REFERENCES snapshots(id),
    head_snapshot_id UUID NOT NULL REFERENCES snapshots(id),
    added_nodes INTEGER NOT NULL,
    removed_nodes INTEGER NOT NULL,
    added_edges INTEGER NOT NULL,
    removed_edges INTEGER NOT NULL,
    storage_ref TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(base_snapshot_id, head_snapshot_id)
);

CREATE TABLE scores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    repo_id UUID NOT NULL REFERENCES repositories(id),
    pr_number INTEGER,
    commit_sha TEXT NOT NULL,
    base_snapshot_id UUID NOT NULL REFERENCES snapshots(id),
    head_snapshot_id UUID NOT NULL REFERENCES snapshots(id),
    delta_id UUID NOT NULL REFERENCES deltas(id),
    total_score REAL NOT NULL,
    grade TEXT NOT NULL,
    breakdown JSONB NOT NULL,
    hotspots JSONB NOT NULL,
    suggested_actions JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ingestions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    repo_id UUID NOT NULL REFERENCES repositories(id),
    commit_sha TEXT NOT NULL,
    pr_number INTEGER,
    status TEXT NOT NULL DEFAULT 'QUEUED',
    error_message TEXT,
    snapshot_id UUID REFERENCES snapshots(id),
    delta_id UUID REFERENCES deltas(id),
    score_id UUID REFERENCES scores(id),
    idempotency_key TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ingestions_status ON ingestions(status) WHERE status != 'COMPLETED';
CREATE INDEX idx_snapshots_repo_branch ON snapshots(repo_id, branch) WHERE branch IS NOT NULL;
CREATE INDEX idx_scores_repo_pr ON scores(repo_id, pr_number);
