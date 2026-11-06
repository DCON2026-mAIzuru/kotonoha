-- Runs only while the dedicated, new Postgres volume is initialized.
-- psql reads passwords from the container environment; no secret is committed.
\getenv postgres_password POSTGRES_PASSWORD
\getenv auth_password SUPABASE_AUTH_DB_PASSWORD

BEGIN;
ALTER USER supabase_admin WITH PASSWORD :'postgres_password';
ALTER USER supabase_auth_admin WITH PASSWORD :'auth_password';

-- The base Postgres image pre-creates these helpers as postgres. GoTrue's
-- initial schema migration replaces them and therefore needs ownership.
ALTER FUNCTION auth.uid() OWNER TO supabase_auth_admin;
ALTER FUNCTION auth.role() OWNER TO supabase_auth_admin;
ALTER FUNCTION auth.email() OWNER TO supabase_auth_admin;

-- These services are not part of this two-container deployment.
-- Optional roles differ between official Postgres image versions.
DO $$
DECLARE unused_role text;
BEGIN
  FOR unused_role IN
    SELECT rolname FROM pg_roles
    WHERE rolname IN ('authenticator', 'pgbouncer', 'supabase_functions_admin', 'supabase_storage_admin')
  LOOP
    EXECUTE format('ALTER ROLE %I NOLOGIN', unused_role);
  END LOOP;
END $$;
COMMIT;
