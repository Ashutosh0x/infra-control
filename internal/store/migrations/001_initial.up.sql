-- Resources table
CREATE TABLE IF NOT EXISTS resources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    provider TEXT NOT NULL,
    region TEXT,
    account TEXT,
    state TEXT NOT NULL DEFAULT 'active',
    tags JSONB DEFAULT '{}',
    configuration JSONB DEFAULT '{}',
    terraform_state JSONB,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    discovered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(external_id)
);
CREATE INDEX idx_resources_provider ON resources(provider);
CREATE INDEX idx_resources_type ON resources(type);
CREATE INDEX idx_resources_state ON resources(state);
CREATE INDEX idx_resources_account ON resources(account);
CREATE INDEX idx_resources_tags ON resources USING GIN(tags);
