-- Orgs module: organisation profile and settings sections. Runs in a tenant transaction
-- (mc.org_id set); every query also names the organisation explicitly.

-- name: CreateOrganisation :exec
INSERT INTO organisations (id, slug, name) VALUES (@org_id, @slug, @name);

-- name: GetOrganisation :one
SELECT id, slug, name, status, default_timezone, default_locale, created_at, updated_at
FROM organisations WHERE id = @org_id;

-- name: UpdateOrganisationProfile :one
-- Optimistic concurrency: with expected_updated_at (from the ETag) the update applies only to
-- the version the client saw.
UPDATE organisations
SET name = @name, default_timezone = @default_timezone, default_locale = @default_locale, updated_at = now()
WHERE id = @org_id
  AND (sqlc.narg(expected_updated_at)::timestamptz IS NULL OR updated_at = sqlc.narg(expected_updated_at)::timestamptz)
RETURNING id, slug, name, status, default_timezone, default_locale, created_at, updated_at;

-- name: GetSettingsSection :one
SELECT settings, version, updated_at FROM organisation_settings
WHERE org_id = @org_id AND section = @section;

-- name: CreateSettingsSection :exec
INSERT INTO organisation_settings (org_id, section, settings, updated_by)
VALUES (@org_id, @section, @settings, @updated_by)
ON CONFLICT (org_id, section) DO NOTHING;

-- name: UpdateSettingsSection :one
-- Optimistic concurrency: the update applies only to the version the client saw.
UPDATE organisation_settings
SET settings = @settings, version = version + 1, updated_by = @updated_by, updated_at = now()
WHERE org_id = @org_id AND section = @section AND version = @expected_version
RETURNING version, updated_at;
