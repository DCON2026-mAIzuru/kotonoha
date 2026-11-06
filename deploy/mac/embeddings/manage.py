#!/usr/bin/env python3
"""Install and manage only this loopback embedding process."""

import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import time
import urllib.error
import urllib.request

from contract import EMBEDDING_SPACE, FILES, HOST, MODEL_ID, PORT, REVISION, SERVICE

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
RUNTIME = ROOT / ".runtime" / "embeddings"
PYTHON = RUNTIME / "venv" / "bin" / "python"
PID_FILE = RUNTIME / "server.pid"
SERVER = HERE / "server.py"
BASE_URL = "http://%s:%s" % (HOST, PORT)
PYTHON_VERSION = "3.12.12"


def digest(path):
    value = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            value.update(block)
    return value.hexdigest()


def install():
    uv = shutil.which("uv") or str(Path.home() / ".local/bin/uv")
    env = dict(os.environ, UV_CACHE_DIR=str(RUNTIME / "uv-cache"),
               UV_PYTHON_INSTALL_DIR=str(RUNTIME / "python"))
    if not PYTHON.exists():
        subprocess.run([uv, "venv", "--python", PYTHON_VERSION, str(RUNTIME / "venv")], env=env, check=True)
    subprocess.run([uv, "pip", "sync", "--python", str(PYTHON), "--require-hashes",
                    str(HERE / "requirements.txt")], env=env, check=True)
    model_dir = RUNTIME / "model"
    model_dir.mkdir(parents=True, exist_ok=True)
    for filename, checksum in FILES.items():
        destination = model_dir / Path(filename).name
        if destination.exists() and digest(destination) == checksum:
            continue
        source = "https://huggingface.co/%s/resolve/%s/%s" % (MODEL_ID, REVISION, filename)
        partial = destination.with_suffix(destination.suffix + ".partial")
        try:
            with urllib.request.urlopen(source, timeout=120) as response, partial.open("wb") as output:
                shutil.copyfileobj(response, output, length=1024 * 1024)
            if digest(partial) != checksum:
                raise RuntimeError("Downloaded model checksum mismatch")
            partial.replace(destination)
        finally:
            if partial.exists():
                partial.unlink()
    print(json.dumps({"installed": True, "python": PYTHON_VERSION,
                      "model_id": MODEL_ID, "revision": REVISION, "embedding_space": EMBEDDING_SPACE}))


def health():
    try:
        with urllib.request.urlopen(BASE_URL + "/health", timeout=2) as response:
            data = json.load(response)
        if data.get("service") == SERVICE and data.get("embedding_space") == EMBEDDING_SPACE:
            return data
    except (OSError, ValueError):
        pass
    return None


def owned_pid():
    try:
        pid = int(PID_FILE.read_text().strip())
    except (OSError, ValueError):
        return None
    check = subprocess.run(["ps", "-p", str(pid), "-o", "command="], capture_output=True, text=True)
    if check.returncode == 0 and str(SERVER) in check.stdout and str(RUNTIME) in check.stdout:
        return pid
    return None


def start():
    if health():
        if not owned_pid():
            raise RuntimeError("A matching service is running without our PID record; it was left intact")
        return status()
    if owned_pid():
        raise RuntimeError("Existing embedding process is not ready; inspect its safe log before restarting")
    if not PYTHON.exists() or any(not (RUNTIME / "model" / Path(name).name).exists() for name in FILES):
        raise RuntimeError("Runtime is not installed; run manage.py install first")
    # Do not start a duplicate process if another service occupies our fixed port.
    import socket
    with socket.socket() as probe:
        if probe.connect_ex((HOST, PORT)) == 0:
            raise RuntimeError("Port 50022 is occupied; the existing listener was left intact")
    fd = os.open(str(RUNTIME / "server.log"), os.O_WRONLY | os.O_CREAT | os.O_APPEND, 0o600)
    with os.fdopen(fd, "ab", buffering=0) as log:
        env = dict(os.environ, PYTHONPYCACHEPREFIX=str(RUNTIME / "pycache"))
        process = subprocess.Popen([str(PYTHON), str(SERVER), "--runtime-dir", str(RUNTIME)],
                                   stdout=log, stderr=subprocess.STDOUT, start_new_session=True,
                                   cwd=str(ROOT), env=env)
    PID_FILE.write_text(str(process.pid) + "\n")
    for _ in range(180):
        if process.poll() is not None:
            PID_FILE.unlink(missing_ok=True)
            raise RuntimeError("Embedding process exited; inspect .runtime/embeddings/server.log")
        if health():
            return status()
        time.sleep(0.5)
    raise RuntimeError("Embedding startup is still pending; process left intact; use status")


def stop():
    pid = owned_pid()
    if pid is None:
        if health():
            raise RuntimeError("Unmanaged matching listener found; no process was stopped")
        PID_FILE.unlink(missing_ok=True)
        print(json.dumps({"stopped": True, "already_stopped": True}))
        return
    os.kill(pid, signal.SIGTERM)
    for _ in range(120):
        if owned_pid() is None:
            PID_FILE.unlink(missing_ok=True)
            print(json.dumps({"stopped": True, "drained_active_requests": True}))
            return
        time.sleep(0.25)
    raise RuntimeError("Graceful shutdown is still pending; no forced kill was sent")


def status():
    result = health()
    pid = owned_pid()
    value = {"running": bool(result), "pid": pid, "base_url": BASE_URL,
             "embedding_space": EMBEDDING_SPACE}
    if result:
        value["health"] = result
    if pid:
        measured = subprocess.run(["ps", "-p", str(pid), "-o", "rss="], capture_output=True, text=True)
        if measured.stdout.strip().isdigit():
            value["rss_mib"] = round(int(measured.stdout) / 1024, 1)
    print(json.dumps(value, ensure_ascii=False, indent=2))
    return bool(result)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("install", "start", "stop", "restart", "status", "verify"))
    args = parser.parse_args()
    RUNTIME.mkdir(parents=True, exist_ok=True)
    with (RUNTIME / "manage.lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        if args.command == "restart":
            stop()
            start()
        elif args.command == "verify":
            subprocess.run([sys.executable, str(HERE / "verify.py")], check=True)
        else:
            result = globals()[args.command]()
            if args.command == "status" and not result:
                sys.exit(1)


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, subprocess.CalledProcessError, OSError) as error:
        print("Embedding service: " + str(error), file=sys.stderr)
        sys.exit(1)
