-- Platform foundation: tenant-context helpers and runtime-role privileges.
-- Roles and the database itself are created by `mcctl db bootstrap` (needs superuser rights).
-- Migrations run as mc_migrator with SET ROLE mc_owner, so every object is owned by mc_owner.

-- +goose Up

-- Tenant context read by row-level security policies (MULTITENANCY.md §2.5). Settings are
-- transaction-local; with no setting the functions return NULL and policies match no rows.
CREATE FUNCTION mc_current_org_id() RETURNS uuid
  LANGUAGE sql STABLE PARALLEL SAFE
  AS $$ SELECT nullif(current_setting('mc.org_id', true), '')::uuid $$;

CREATE FUNCTION mc_current_user_id() RETURNS uuid
  LANGUAGE sql STABLE PARALLEL SAFE
  AS $$ SELECT nullif(current_setting('mc.user_id', true), '')::uuid $$;

-- The runtime role reads and writes data but never changes schema or bypasses RLS.
GRANT USAGE ON SCHEMA public TO mc_app;
ALTER DEFAULT PRIVILEGES FOR ROLE mc_owner IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO mc_app;
ALTER DEFAULT PRIVILEGES FOR ROLE mc_owner IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO mc_app;
ALTER DEFAULT PRIVILEGES FOR ROLE mc_owner IN SCHEMA public
  GRANT EXECUTE ON FUNCTIONS TO mc_app;

GRANT EXECUTE ON FUNCTION mc_current_org_id(), mc_current_user_id() TO mc_app;

-- The API and worker compare the applied version with their build at start-up.
GRANT SELECT ON goose_db_version TO mc_app;

-- +goose Down

REVOKE SELECT ON goose_db_version FROM mc_app;
ALTER DEFAULT PRIVILEGES FOR ROLE mc_owner IN SCHEMA public
  REVOKE EXECUTE ON FUNCTIONS FROM mc_app;
ALTER DEFAULT PRIVILEGES FOR ROLE mc_owner IN SCHEMA public
  REVOKE USAGE, SELECT ON SEQUENCES FROM mc_app;
ALTER DEFAULT PRIVILEGES FOR ROLE mc_owner IN SCHEMA public
  REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM mc_app;
REVOKE USAGE ON SCHEMA public FROM mc_app;
DROP FUNCTION mc_current_user_id();
DROP FUNCTION mc_current_org_id();
