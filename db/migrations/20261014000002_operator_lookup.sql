-- Operator tooling (mcctl) names organisations by slug but has no tenant context, and
-- organisations are protected by row-level security. This narrow lookup returns only the ID.

-- +goose Up

-- +goose StatementBegin
CREATE FUNCTION mc_org_id_by_slug(p_slug text)
RETURNS uuid
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
AS $$
  SELECT id FROM organisations WHERE lower(slug) = lower(p_slug) AND status <> 'deleting'
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION mc_org_id_by_slug(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION mc_org_id_by_slug(text) TO mc_app;

-- +goose Down

DROP FUNCTION mc_org_id_by_slug(text);
