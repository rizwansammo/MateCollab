-- Audit log CSV exports (BACKEND.md: `audit` module, `audit.export` job on the maintenance
-- queue; FILES.md: objects under orgs/{orgId}/exports/, expiring after 7 days).

-- +goose Up

CREATE TABLE audit_exports (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id       uuid NOT NULL REFERENCES organisations (id),
  requested_by uuid NOT NULL,
  filters      jsonb NOT NULL DEFAULT '{}',
  status       text NOT NULL DEFAULT 'pending'
               CHECK (status IN ('pending', 'running', 'completed', 'failed', 'expired')),
  object_key   text CHECK (char_length(object_key) <= 200),
  row_count    int,
  error        text CHECK (char_length(error) <= 200),
  created_at   timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz,
  expires_at   timestamptz NOT NULL,
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, requested_by) REFERENCES organisation_memberships (org_id, user_id)
);
CREATE INDEX audit_exports_org_created ON audit_exports (org_id, created_at DESC);

ALTER TABLE audit_exports ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_exports FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON audit_exports
  USING (org_id = mc_current_org_id()) WITH CHECK (org_id = mc_current_org_id());
REVOKE DELETE, TRUNCATE ON audit_exports FROM mc_app;

-- +goose Down

DROP TABLE audit_exports;
