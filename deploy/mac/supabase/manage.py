#!/usr/bin/env python3
"""Manage the Mac-only Postgres/Auth services without printing secrets."""

import argparse
import base64
import hashlib
import hmac
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request


HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
RUNTIME = ROOT / ".runtime"
ENV_FILE = RUNTIME / "supabase.env"
USER_FILE = RUNTIME / "supabase-demo-user.json"
AUTH_URL = "http://127.0.0.1:54321"
MIGRATIONS = ROOT / "supabase" / "migrations"


def private_write(path, content):
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    fd, temporary = tempfile.mkstemp(prefix=".private-", dir=path.parent)
    try:
        os.fchmod(fd, 0o600)
        with os.fdopen(fd, "w") as stream:
            stream.write(content)
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def read_env():
    if not ENV_FILE.exists():
        raise RuntimeError("Run prepare before starting Supabase.")
    result = {}
    for line in ENV_FILE.read_text().splitlines():
        if line and not line.startswith("#"):
            key, value = line.split("=", 1)
            result[key] = value
    return result


def jwt_token(secret, role, issuer):
    def encode(value):
        return base64.urlsafe_b64encode(json.dumps(value, separators=(",", ":")).encode()).rstrip(b"=")

    now = int(time.time())
    header = encode({"alg": "HS256", "typ": "JWT"})
    payload = encode({"role": role, "iss": issuer, "iat": now, "exp": now + 365 * 24 * 3600})
    message = header + b"." + payload
    signature = base64.urlsafe_b64encode(hmac.new(secret.encode(), message, hashlib.sha256).digest()).rstrip(b"=")
    return (message + b"." + signature).decode()


def prepare(site_url):
    parsed = urllib.parse.urlsplit(site_url)
    if parsed.scheme not in ("https", "http") or not parsed.hostname or parsed.path not in ("", "/") or parsed.query or parsed.fragment or parsed.username:
        raise RuntimeError("--site-url must be a complete origin with no path, credentials, query or fragment.")
    if parsed.scheme == "http" and parsed.hostname not in ("localhost", "127.0.0.1", "::1"):
        raise RuntimeError("A public site origin must use HTTPS.")
    origin = site_url.rstrip("/")
    values = read_env() if ENV_FILE.exists() else {
        "POSTGRES_PASSWORD": secrets.token_hex(32),
        "SUPABASE_AUTH_DB_PASSWORD": secrets.token_hex(32),
        "JWT_SECRET": secrets.token_hex(64),
    }
    values["SITE_URL"] = origin
    values["API_EXTERNAL_URL"] = origin + "/supabase/auth/v1"
    # Direct Auth accepts the legacy HS256 keys. Opaque sb_ API keys would
    # require an API gateway, which this minimal deployment does not run.
    if "ANON_KEY" not in values:
        values["ANON_KEY"] = jwt_token(values["JWT_SECRET"], "anon", values["API_EXTERNAL_URL"])
    if "SERVICE_ROLE_KEY" not in values:
        values["SERVICE_ROLE_KEY"] = jwt_token(values["JWT_SECRET"], "service_role", values["API_EXTERNAL_URL"])
    private_write(ENV_FILE, "# Private runtime configuration. Never commit or print.\n" + "".join(f"{key}={value}\n" for key, value in values.items()))
    print("Supabase configuration prepared; existing secrets preserved.")


def docker_command():
    executable = shutil.which("docker")
    if not executable:
        candidate = Path.home() / ".orbstack" / "bin" / "docker"
        if candidate.exists():
            executable = str(candidate)
    if not executable:
        raise RuntimeError("Docker CLI is unavailable. Start OrbStack first.")
    return [executable, "compose", "--env-file", str(ENV_FILE), "-f", str(HERE / "compose.yaml")]


def compose(*arguments, input_text=None, capture=False):
    environment = {**os.environ, **read_env()}
    try:
        result = subprocess.run(docker_command() + list(arguments), input=input_text, text=True,
                                stdout=subprocess.PIPE if capture else None, stderr=subprocess.PIPE,
                                env=environment, timeout=600 if arguments[0] == "up" else 60)
    except subprocess.TimeoutExpired:
        raise RuntimeError("Docker did not respond before the timeout. Check OrbStack before retrying; database volumes have not been deleted.") from None
    if result.returncode:
        # Docker error details can include interpolated connection strings.
        # Keep them in the private runtime directory, never on the console.
        private_write(RUNTIME / "supabase-error.log", result.stderr or "Command failed without stderr.\n")
        raise RuntimeError("Supabase command failed; details are in .runtime/supabase-error.log.")
    return result.stdout or ""


def sql(query):
    return compose("exec", "-T", "db", "psql", "-X", "-qAt", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", "postgres",
                   input_text=query, capture=True).strip()


def migrate_new():
    files = sorted(MIGRATIONS.glob("*.sql"))
    expected = {path.name: hashlib.sha256(path.read_bytes()).hexdigest() for path in files}
    if not expected:
        raise RuntimeError("No application migrations found.")
    initialized = sql("SELECT to_regclass('kotonoha_deploy.migrations') IS NOT NULL;") == "t"
    if initialized:
        current = dict(line.split("|", 1) for line in sql("SELECT filename, checksum FROM kotonoha_deploy.migrations ORDER BY filename;").splitlines())
        if current != expected:
            raise RuntimeError("Migration files differ from this initialized database. Use a reviewed incremental migration; full replay is refused.")
        print("Application schema is already initialized; no migrations reapplied.")
        return

    if sql("SELECT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename IN ('bonsai_profiles','chat_history','bonsai_memories','sensor_data','push_subscriptions','bonsai_diagnoses'));") != "f":
        raise RuntimeError("Existing application tables found without deployment history. Refusing destructive migration replay.")

    statements = ["BEGIN;", "SELECT pg_advisory_xact_lock(1263817800);", """
DO $$ BEGIN
  IF to_regclass('public.bonsai_profiles') IS NOT NULL THEN
    RAISE EXCEPTION 'Existing application database; migration replay refused';
  END IF;
END $$;
CREATE SCHEMA kotonoha_deploy;
REVOKE ALL ON SCHEMA kotonoha_deploy FROM PUBLIC;
CREATE TABLE kotonoha_deploy.migrations (filename text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now());
"""
    ]
    for path in files:
        # Existing files include their own transactions. A fresh installation
        # is applied atomically under the single transaction above.
        source = re.sub(r"^\s*(?:begin|commit);\s*$", "", path.read_text(), flags=re.IGNORECASE | re.MULTILINE)
        statements.append(source)
        filename = path.name.replace("'", "''")
        statements.append(f"INSERT INTO kotonoha_deploy.migrations (filename, checksum) VALUES ('{filename}', '{expected[path.name]}');")
    statements.append("COMMIT;")
    sql("\n".join(statements))
    count = sql("SELECT count(*) FROM kotonoha_deploy.migrations;")
    if count != str(len(files)):
        raise RuntimeError("Unexpected migration count after initialization.")
    print(f"Initialized {count} application migrations on the new database.")


def auth_request(path, method="GET", body=None, token=None):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    request = urllib.request.Request(AUTH_URL + path, method=method, headers=headers,
                                     data=json.dumps(body).encode() if body is not None else None)
    try:
        with urllib.request.urlopen(request, timeout=20) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        raise RuntimeError(f"Local Auth request failed with HTTP {error.code}.") from None
    except urllib.error.URLError:
        raise RuntimeError("Local Auth is unreachable; start the services first.") from None


def create_demo_user():
    values = read_env()
    if USER_FILE.exists():
        account = json.loads(USER_FILE.read_text())
        user = auth_request("/admin/users/" + account["user_id"], token=values["SERVICE_ROLE_KEY"])
        if user.get("id") != account["user_id"]:
            raise RuntimeError("Stored demo user does not match the running database.")
        print("Existing demo user retained; credentials remain in the private runtime file.")
        return
    account = {"email": "demo@kotonoha.local", "password": secrets.token_urlsafe(24)}
    user = auth_request("/admin/users", method="POST", token=values["SERVICE_ROLE_KEY"],
                        body={**account, "email_confirm": True})
    if not user.get("id"):
        raise RuntimeError("Auth did not return the created user's ID.")
    account["user_id"] = user["id"]
    account["created_at"] = int(time.time())
    private_write(USER_FILE, json.dumps(account, ensure_ascii=False, indent=2) + "\n")
    # Verify password authentication locally; never persist or print access or
    # refresh tokens returned by this probe.
    session = auth_request("/token?grant_type=password", method="POST",
                           body={"email": account["email"], "password": account["password"]})
    if session.get("user", {}).get("id") != account["user_id"] or not session.get("access_token"):
        raise RuntimeError("Demo account was created but local password login failed.")
    print("Demo user created and local password login verified; credentials saved privately.")


def status():
    health = auth_request("/health")
    settings = auth_request("/settings")
    database = sql("SELECT 1;") == "1"
    print(json.dumps({"auth_reachable": bool(health), "database_reachable": database,
                      "public_signup_enabled": not settings.get("disable_signup", True),
                      "smtp_configured": False,
                      "google_oauth_configured": settings.get("external", {}).get("google", False)}, ensure_ascii=False))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="action", required=True)
    prepare_parser = sub.add_parser("prepare", help="Generate private secrets once; configure the public origin.")
    prepare_parser.add_argument("--site-url", required=True)
    for action in ("start", "stop", "status", "migrate-new", "create-demo-user"):
        sub.add_parser(action)
    args = parser.parse_args()
    try:
        if args.action == "prepare":
            prepare(args.site_url)
        elif args.action == "start":
            compose("up", "-d", "--wait", "--wait-timeout", "180", "db", "auth")
            status()
        elif args.action == "stop":
            compose("stop", "auth", "db")
            print("Supabase services stopped; database volumes and private configuration preserved.")
        elif args.action == "migrate-new":
            migrate_new()
        elif args.action == "create-demo-user":
            create_demo_user()
        else:
            status()
    except (RuntimeError, ValueError, OSError) as error:
        # Exceptions handled here are controlled messages, without secret values.
        print(str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
