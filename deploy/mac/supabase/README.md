# Mac Postgres and Supabase Auth

This deployment runs only the official `supabase/postgres:17.6.1.136` and
`supabase/gotrue:v2.197.0` images. It is separate from the Supabase CLI development
stack. Kotonoha connects to Postgres directly, so PostgREST, Studio, Realtime,
Storage, Edge Functions and the Supabase API gateway are not started.

Run these commands from the repository root after starting OrbStack:

```sh
python3 deploy/mac/supabase/manage.py prepare --site-url https://YOUR-PUBLIC-HOST
python3 deploy/mac/supabase/manage.py start
python3 deploy/mac/supabase/manage.py migrate-new
python3 deploy/mac/supabase/manage.py create-demo-user
python3 deploy/mac/supabase/manage.py status
```

`prepare` generates random database passwords, a JWT signing secret and legacy
`anon` / `service_role` API keys in `.runtime/supabase.env`, with mode `0600`.
Repeated preparation preserves those secrets. Do not print that file, commit it,
or run `docker compose config` without `--quiet`, because expanded Compose output
contains secrets. The repository ignores `.runtime/`.

`migrate-new` applies the existing application migrations in one transaction to
an empty application database. It stores migration names and checksums in the
private `kotonoha_deploy` schema. A repeat with the same migrations does nothing.
Existing application tables without that deployment history, or changed
migration files, cause an error. This protects the existing migration that
truncates vector memories from being replayed against an established database.
Future migrations need a separately reviewed incremental deployment.

`create-demo-user` creates one confirmed local demonstration account, verifies
password login, and writes its email, random password and user ID to
`.runtime/supabase-demo-user.json`, also with mode `0600`. It never prints the
credentials. Repeating the command preserves an existing demo account. Open the
private file locally when signing in; do not copy it into source control.

Public sign-up, anonymous users, phone sign-in and Google OAuth are disabled.
SMTP is unconfigured, so email confirmation, invitation and password recovery
flows are unavailable. An administrator can create or manage accounts through
the loopback Auth endpoint. The application should present password sign-in as
the available method.

## Routing and application configuration

- Postgres is bound only to `127.0.0.1:54322`.
- Auth is bound only to `127.0.0.1:54321`, with native routes such as `/token`,
  `/user`, `/settings`, `/logout`, `/health` and `/admin/users`.
- The Mac HTTPS reverse proxy must map `/supabase/auth/v1/...` to Auth by
  **removing the complete `/supabase/auth/v1` prefix**. There is no API gateway
  to remove `/auth/v1` a second time.
- Publish only the endpoints needed for password sign-in and session refresh.
  In particular, `/admin` is local administration only. Do not expose Postgres.
- Preserve the request's user Bearer token. Do not inject the service-role key
  into publicly routed requests.

Application environment mapping:

| Application setting | Value |
| --- | --- |
| `NEXT_PUBLIC_SUPABASE_URL` | Public origin followed by `/supabase` |
| `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY` | `ANON_KEY` from the private configuration |
| `SUPABASE_URL` | `http://127.0.0.1:8787/supabase` when using the Mac proxy |
| `SUPABASE_PUBLISHABLE_KEY` | `ANON_KEY` |
| `SUPABASE_SECRET_KEY` | `SERVICE_ROLE_KEY`, server side only |
| `SUPABASE_JWT_SECRET` | Empty: the backend verifies sessions through Auth `/user` |
| `SUPABASE_DB_URL` | `postgresql://postgres:<POSTGRES_PASSWORD>@127.0.0.1:54322/postgres?sslmode=disable` |

The two legacy API-key environment names are deliberate: opaque `sb_` keys
require a Supabase gateway. This app's existing variable names accept a legacy
anon key. Its value is public; the service-role key is not.

The Auth issuer and external URL use the public origin plus
`/supabase/auth/v1`. A changed public origin requires running `prepare` with the
new origin, restarting Auth, updating the frontend URL and rebuilding Next.js.

## Stop, restart and data

```sh
python3 deploy/mac/supabase/manage.py stop
python3 deploy/mac/supabase/manage.py start
```

Stopping preserves the `kotonoha-supabase` project volumes and private
configuration. Do not remove its volumes or delete the private configuration to
restart it. The generated database passwords must stay with their database.
There is no automatic backup or deletion command in this helper.

If a Compose command fails, inspect `.runtime/supabase-error.log` locally. That
file can contain sensitive connection details and must not be pasted into chat.

Upstream references:

- [Official self-hosted Compose](https://github.com/supabase/supabase/blob/master/docker/docker-compose.yml)
- [Standalone Supabase Auth](https://github.com/supabase/auth)
- [Self-hosted Auth settings](https://supabase.com/docs/guides/self-hosting/auth/config)
