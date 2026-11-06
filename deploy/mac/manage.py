#!/usr/bin/env python3
"""Macでのビルド・起動・停止。秘密値は表示せず、Colabの操作は行わない。"""
import argparse
import json
import os
from pathlib import Path
import shlex
import shutil
import signal
import socket
import subprocess
import sys
import time
import urllib.error
import urllib.request
import urllib.parse

ROOT = Path(__file__).resolve().parents[2]
STATE = ROOT / '.runtime'
SCRIPT = Path(__file__).resolve()


def read_env(path):
    values = {}
    if path.exists():
        for line in path.read_text().splitlines():
            line = line.strip()
            if not line or line.startswith('#') or '=' not in line:
                continue
            key, value = line.removeprefix('export ').split('=', 1)
            tokens = shlex.split(value, comments=True)
            values[key.strip()] = ' '.join(tokens)
    return values


def configured():
    backend = read_env(ROOT / 'backend/.env')
    frontend = read_env(ROOT / 'frontend/.env.local')
    required = {
        'backend/.env': (backend, ['SUPABASE_URL', 'SUPABASE_PUBLISHABLE_KEY', 'SUPABASE_SECRET_KEY', 'SUPABASE_DB_URL', 'BACKEND_ALLOWED_ORIGIN', 'LLM_PROVIDER']),
        'frontend/.env.local': (frontend, ['NEXT_PUBLIC_SUPABASE_URL', 'NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY']),
    }
    missing = []
    for file, (values, keys) in required.items():
        missing += [file + ': ' + key for key in keys if not values.get(key) or 'your-' in values[key]]
    if missing:
        raise RuntimeError('未設定の項目:\n' + '\n'.join(missing))
    if backend['LLM_PROVIDER'] not in ['openai-compatible', 'disabled']:
        raise RuntimeError('このMac構成は LLM_PROVIDER=openai-compatible または disabled を使用します')
    if backend['LLM_PROVIDER'] == 'openai-compatible':
        for key in ['LLM_BASE_URL', 'LLM_MODEL', 'LLM_API_KEY']:
            if not backend.get(key):
                raise RuntimeError(key + ' が未設定です')
    for public_name, private_name in [('NEXT_PUBLIC_SUPABASE_URL', 'SUPABASE_URL'), ('NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY', 'SUPABASE_PUBLISHABLE_KEY')]:
        local_auth = public_name == 'NEXT_PUBLIC_SUPABASE_URL' and backend[private_name] == 'http://127.0.0.1:8787/supabase' and frontend[public_name] == backend['BACKEND_ALLOWED_ORIGIN'].rstrip('/') + '/supabase'
        if frontend[public_name] != backend[private_name] and not local_auth:
            raise RuntimeError(public_name + ' がバックエンドの設定と一致しません')
    for key in ['NEXT_PUBLIC_API_BASE_URL', 'NEXT_PUBLIC_WS_URL']:
        if frontend.get(key):
            raise RuntimeError(key + ' は同一オリジン構成では空にしてください')
    return backend


def http_ok(url):
    try:
        with urllib.request.urlopen(url, timeout=2) as response:
            return response.status == 200
    except (OSError, urllib.error.URLError):
        return False


def active_pid():
    try:
        pid = int((STATE / 'supervisor.pid').read_text())
        command = subprocess.check_output(['ps', '-p', str(pid), '-o', 'command='], text=True, stderr=subprocess.DEVNULL)
        return pid if str(SCRIPT) in command and 'serve' in command else None
    except (ValueError, OSError, subprocess.CalledProcessError):
        return None


def build():
    if active_pid():
        raise RuntimeError('稼働中のビルド上書きを避けるため、先に stop を実行してください')
    for command in ['go', 'node', 'npm']:
        if not shutil.which(command):
            raise RuntimeError(command + ' が見つかりません')
    subprocess.run(['go', 'build', '-o', str(STATE / 'backend'), './cmd/server'], cwd=ROOT / 'backend', check=True)
    subprocess.run(['go', 'build', '-o', str(STATE / 'proxy'), str(SCRIPT.parent / 'proxy.go')], cwd=ROOT, check=True)
    subprocess.run(['npm', 'run', 'build'], cwd=ROOT / 'frontend', check=True)
    print('Mac用ビルド完了')


def serve():
    configured()
    if active_pid():
        raise RuntimeError('すでに起動しています')
    for port in [3000, 8080, 8787]:
        with socket.socket() as sock:
            # Ignore TIME_WAIT sockets left by the previous server, while still
            # rejecting a port with an active listener.
            sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            sock.bind(('127.0.0.1', port))
    children = []
    stopping = False
    def stop(_signal, _frame):
        nonlocal stopping
        stopping = True
    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    (STATE / 'supervisor.pid').write_text(str(os.getpid()))
    base_env = dict(os.environ)
    base_env['NODE_ENV'] = 'production'
    base_env['NEXT_TELEMETRY_DISABLED'] = '1'
    backend_env = dict(base_env, BACKEND_HOST='127.0.0.1', BACKEND_PORT='8080', GIN_MODE='release')
    processes = [
        ('backend', [str(STATE / 'backend')], ROOT / 'backend', backend_env, 'http://127.0.0.1:8080/readyz'),
        ('frontend', [shutil.which('node'), 'node_modules/next/dist/bin/next', 'start', '--hostname', '127.0.0.1', '--port', '3000'], ROOT / 'frontend', base_env, 'http://127.0.0.1:3000/'),
        ('proxy', [str(STATE / 'proxy')], ROOT, base_env, 'http://127.0.0.1:8787/readyz'),
    ]
    try:
        for name, command, cwd, env, health in processes:
            with (STATE / (name + '.log')).open('ab') as log:
                child = subprocess.Popen(command, cwd=cwd, env=env, stdout=log, stderr=log, start_new_session=True)
            children.append(child)
            deadline = time.monotonic() + 45
            while not http_ok(health):
                if stopping or child.poll() is not None or time.monotonic() > deadline:
                    raise RuntimeError(name + ' の起動確認に失敗しました。.runtime 内のログを確認してください')
                time.sleep(0.3)
        print('Mac上で起動しました: http://127.0.0.1:8787', flush=True)
        while not stopping:
            if any(child.poll() is not None for child in children):
                raise RuntimeError('サービスが停止したため、関連プロセスを停止します')
            time.sleep(1)
    finally:
        try:
            for child in reversed(children):
                if child.poll() is None:
                    try:
                        os.killpg(child.pid, signal.SIGTERM)
                    except ProcessLookupError:
                        pass
            for child in children:
                try:
                    child.wait(timeout=12)
                except subprocess.TimeoutExpired:
                    try:
                        os.killpg(child.pid, signal.SIGKILL)
                    except ProcessLookupError:
                        pass
                    child.wait()
        finally:
            (STATE / 'supervisor.pid').unlink(missing_ok=True)


def main():
    os.umask(0o077)
    STATE.mkdir(exist_ok=True, mode=0o700)
    parser = argparse.ArgumentParser()
    parser.add_argument('action', choices=['check', 'build', 'start', 'serve', 'stop', 'status', 'check-llm'])
    action = parser.parse_args().action
    if action == 'build':
        build()
    elif action == 'check':
        configured()
        print('必須設定と接続先の整合性: OK（実接続は未確認）')
    elif action == 'serve':
        serve()
    elif action == 'start':
        configured()
        if active_pid():
            raise RuntimeError('すでに起動しています')
        for path in [STATE / 'backend', STATE / 'proxy', ROOT / 'frontend/.next/BUILD_ID']:
            if not path.exists():
                raise RuntimeError('先に build を実行してください')
        with (STATE / 'supervisor.log').open('ab') as log:
            subprocess.Popen([sys.executable, str(SCRIPT), 'serve'], cwd=ROOT, stdout=log, stderr=log, start_new_session=True)
        print('起動を開始しました。status で確認してください')
    elif action == 'stop':
        pid = active_pid()
        if pid:
            os.kill(pid, signal.SIGTERM)
            print('MacのWeb/API停止を要求しました。Colabは変更しません')
        else:
            print('管理対象のプロセスは起動していません')
    elif action == 'status':
        print('管理プロセス: ' + ('起動中' if active_pid() else '停止'))
        for label, url in [('Web', 'http://127.0.0.1:8787/'), ('API/DB', 'http://127.0.0.1:8787/readyz'), ('VOICEVOX', 'http://127.0.0.1:50021/version'), ('意味検索モデル', 'http://127.0.0.1:50022/health')]:
            print(label + ': ' + ('応答あり' if http_ok(url) else '応答なし'))
    elif action == 'check-llm':
        env = read_env(ROOT / 'backend/.env')
        for key in ['LLM_BASE_URL', 'LLM_MODEL', 'LLM_API_KEY']:
            if not env.get(key):
                raise RuntimeError(key + ' が未設定です')
        base = env['LLM_BASE_URL'].rstrip('/')
        parsed = urllib.parse.urlsplit(base)
        if parsed.username or parsed.password or parsed.query or parsed.fragment or not parsed.hostname:
            raise RuntimeError('LLM_BASE_URL は認証情報やクエリを含まないURLにしてください')
        if parsed.scheme != 'https' and not (parsed.scheme == 'http' and parsed.hostname in ['127.0.0.1', 'localhost', '::1']):
            raise RuntimeError('LLM_BASE_URL はHTTPSまたはローカルHTTPを使用してください')
        class NoRedirect(urllib.request.HTTPRedirectHandler):
            def redirect_request(self, req, fp, code, msg, headers, newurl):
                return None
        opener = urllib.request.build_opener(NoRedirect)
        payload = json.dumps({'model': env['LLM_MODEL'], 'messages': [{'role': 'user', 'content': '日本語で短く挨拶してください。'}], 'max_tokens': 128, 'stream': True}).encode()
        req = urllib.request.Request(base + '/chat/completions', data=payload, headers={'Authorization': 'Bearer ' + env['LLM_API_KEY'], 'Content-Type': 'application/json', 'ngrok-skip-browser-warning': 'true'})
        count, done = 0, False
        try:
            with opener.open(req, timeout=120) as response:
                for raw in response:
                    line = raw.decode().strip()
                    if line == 'data: [DONE]':
                        done = True
                        break
                    if line.startswith('data: '):
                        event = json.loads(line[6:])
                        if event.get('error'):
                            raise RuntimeError('LLMがエラーを返しました（本文は秘密保護のため表示しません）')
                        for choice in event.get('choices', []):
                            count += bool(choice.get('delta', {}).get('content'))
            if not count or not done:
                raise RuntimeError('LLMストリームが正常完了しませんでした')
            print('LLMへの実リクエスト: OK / 内容チャンク数=' + str(count) + ' / [DONE]確認済み')
        except urllib.error.HTTPError as exc:
            raise RuntimeError('LLM HTTPエラー: ' + str(exc.code)) from None
        except urllib.error.URLError:
            raise RuntimeError('LLMへ接続できません。Colabとトンネルを確認してください') from None


if __name__ == '__main__':
    try:
        main()
    except (RuntimeError, OSError, subprocess.CalledProcessError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
