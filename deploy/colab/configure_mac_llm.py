#!/usr/bin/env python3
"""Verify a restored HTTPS LLM endpoint, then update the Mac backend .env."""
import argparse
import getpass
import json
import os
from pathlib import Path
import re
import tempfile
import urllib.error
import urllib.parse
import urllib.request

class NoRedirects(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base-url', required=True)
    parser.add_argument('--model', default='qwen38-27b')
    parser.add_argument('--key-file', type=Path)
    parser.add_argument('--env-file', type=Path, required=True)
    args = parser.parse_args()
    base = args.base_url.rstrip('/')
    parsed = urllib.parse.urlsplit(base)
    if parsed.scheme != 'https' or not parsed.hostname or (parsed.username is not None or parsed.password is not None) or parsed.query or parsed.fragment:
        raise ValueError('Use the HTTPS API base URL without credentials, query, or fragment.')
    key = args.key_file.read_text().strip() if args.key_file else getpass.getpass('Existing VLLM API key (hidden): ')
    if not key or '\n' in key or '\r' in key:
        raise ValueError('A nonempty single-line key is required.')
    opener = urllib.request.build_opener(NoRedirects())
    def request(path, payload=None, authenticated=True):
        headers = {'ngrok-skip-browser-warning': '1'}
        if authenticated:
            headers['Authorization'] = 'Bearer ' + key
        data = None
        if payload is not None:
            headers['Content-Type'] = 'application/json'
            data = json.dumps(payload, ensure_ascii=False).encode()
        return opener.open(urllib.request.Request(base + path, data=data, headers=headers), timeout=180)
    try:
        with request('/models', authenticated=False):
            raise RuntimeError('Endpoint accepts unauthenticated model access; verify server authentication first.')
    except urllib.error.HTTPError as error:
        if error.code not in (401, 403):
            raise RuntimeError('Endpoint is not ready for authentication verification.') from None
    with request('/models') as response:
        model_ids = [item['id'] for item in json.load(response).get('data', [])]
    if args.model not in model_ids:
        raise RuntimeError('Expected model ID not returned; confirm /v1/models before continuing.')
    content = []
    complete = False
    payload = {'model': args.model, 'messages': [{'role': 'user', 'content': '日本語で短く挨拶してください。'}],
               'stream': True, 'max_tokens': 128, 'temperature': 0.2,
               'chat_template_kwargs': {'enable_thinking': False}}
    with request('/chat/completions', payload) as response:
        if 'text/event-stream' not in response.headers.get('Content-Type', ''):
            raise RuntimeError('Expected a streaming SSE response.')
        for line in response:
            line = line.decode('utf-8').strip()
            if not line.startswith('data:'):
                continue
            data = line[5:].strip()
            if data == '[DONE]':
                complete = True
                break
            event = json.loads(data)
            if 'error' in event:
                raise RuntimeError('Upstream returned a streaming error.')
            for choice in event.get('choices', []):
                piece = choice.get('delta', {}).get('content')
                if piece:
                    content.append(piece)
    if not complete or not ''.join(content).strip():
        raise RuntimeError('No complete nonempty assistant response; configuration was not saved.')
    values = {'LLM_PROVIDER': 'openai-compatible', 'LLM_BASE_URL': base,
              'LLM_MODEL': args.model, 'LLM_API_KEY': key}
    # Go's godotenv double-quoted form handles escaping without command execution.
    def quote(value):
        return '"' + value.replace('\\', '\\\\').replace('"', '\\"').replace('$', '\\$') + '"'
    path = args.env_file.resolve()
    path.parent.mkdir(parents=True, exist_ok=True)
    original = path.read_text() if path.exists() else ''
    lines = [line for line in original.splitlines()
             if not re.match(r'^\s*(?:export\s+)?(?:' + '|'.join(values) + r')\s*=', line)]
    lines.extend(f'{name}={quote(value)}' for name, value in values.items())
    fd, temporary = tempfile.mkstemp(prefix='.llm-env-', dir=path.parent)
    try:
        os.fchmod(fd, 0o600)
        with os.fdopen(fd, 'w') as destination:
            destination.write('\n'.join(lines) + '\n')
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)
    print('PASS: authentication required; model verified; nonempty SSE stream completed.')
    print(f'Updated {path} (0600). Key and response text were not printed.')

if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        # Never emit response bodies, request headers, or credential-containing URLs.
        if isinstance(error, urllib.error.HTTPError):
            print(f'Connection verification failed: HTTP {error.code}. Configuration was not saved.')
        else:
            print(f'Connection verification failed: {type(error).__name__}. Configuration was not saved.')
        raise SystemExit(1)
