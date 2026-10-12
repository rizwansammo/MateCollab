# db/queries

SQL queries compiled to typed Go by sqlc (ADR-003), one directory per backend module
(`db/queries/<module>/*.sql`). Queries arrive with the first product data (authentication and
organisations); see [DATABASE.md §9](../../docs/architecture/DATABASE.md#9-sqlc-conventions).
Every tenant-owned query carries an explicit `org_id` predicate in addition to RLS.
