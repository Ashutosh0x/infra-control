CREATE TABLE IF NOT EXISTS drift_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_id UUID NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    severity TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    details JSONB DEFAULT '{}',
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX idx_drift_events_severity ON drift_events(severity);
CREATE INDEX idx_drift_events_resource_id ON drift_events(resource_id);
CREATE INDEX idx_drift_events_detected_at ON drift_events(detected_at);
CREATE INDEX idx_drift_events_resolved_at ON drift_events(resolved_at);
CREATE INDEX idx_drift_events_status ON drift_events(status);
