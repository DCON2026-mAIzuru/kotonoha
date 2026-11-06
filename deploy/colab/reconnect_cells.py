# %% 1. Existing runtime inspection. Execute cells individually in the SAME notebook.
# Never use Run all on the original notebook: its first code cell terminates a server.
import os
import subprocess
import urllib.request

vram_mib = int(subprocess.check_output([
    'nvidia-smi', '--query-gpu=memory.total', '--format=csv,noheader,nounits'
], text=True).split()[0])
gpu_name = subprocess.check_output([
    'nvidia-smi', '--query-gpu=name', '--format=csv,noheader'
], text=True).strip()
if 'A100' not in gpu_name:
    raise RuntimeError('Expected the approved A100 runtime; do not allocate another runtime.')
MODEL = 'Qwen/Qwen3.8-27B' if vram_mib >= 70000 else 'Qwen/Qwen3.8-27B-FP8'
MAX_LEN = 131072 if vram_mib >= 70000 else 65536
SERVED_MODEL = 'qwen38-27b'
PORT = 8000
try:
    with urllib.request.urlopen('http://127.0.0.1:8000/health', timeout=3) as response:
        SERVER_ALREADY_RUNNING = response.status == 200
except Exception:
    SERVER_ALREADY_RUNNING = False
print({'gpu': gpu_name, 'model': MODEL, 'served_model': SERVED_MODEL,
       'server_already_running': SERVER_ALREADY_RUNNING})

# %% 2. Dependencies; skip installation if the existing server is healthy.
import importlib.util
import sys
needed = ['openai', 'ngrok'] + ([] if SERVER_ALREADY_RUNNING else ['vllm'])
missing = [name for name in needed if importlib.util.find_spec(name) is None]
if missing:
    subprocess.run([sys.executable, '-m', 'pip', 'install', '-q', *missing], check=True)
print('Dependency check complete. Do not restart an existing runtime automatically.')

# %% 3. Key reuse and vLLM start. Reuse keys already present in the kernel or environment.
# Do not place literal keys into notebook source or output.
import socket
from pathlib import Path
VLLM_API_KEY = globals().get('VLLM_API_KEY') or os.environ.get('VLLM_API_KEY')
NGROK_AUTHTOKEN = globals().get('NGROK_AUTHTOKEN') or os.environ.get('NGROK_AUTHTOKEN')
if not VLLM_API_KEY or not NGROK_AUTHTOKEN:
    from google.colab import userdata
    if not VLLM_API_KEY:
        VLLM_API_KEY = userdata.get('VLLM_API_KEY')
    if not NGROK_AUTHTOKEN:
        NGROK_AUTHTOKEN = userdata.get('NGROK_AUTHTOKEN')
if not VLLM_API_KEY or not NGROK_AUTHTOKEN:
    raise RuntimeError('Missing secret: save it in Colab Secrets and enable notebook access; never print it.')
if not SERVER_ALREADY_RUNNING:
    # pgrep emits only PIDs; never print process arguments containing the API key.
    existing_vllm = subprocess.run(
        ['pgrep', '-f', r'(^|[ /])vllm[[:space:]]+serve([[:space:]]|$)'],
        stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True,
    )
    if existing_vllm.returncode not in (0, 1):
        raise RuntimeError('Could not inspect existing vLLM processes; do not start another server.')
    if existing_vllm.returncode == 0:
        print({'existing_vllm_pids': [int(pid) for pid in existing_vllm.stdout.split() if pid.isdigit()],
               'status': 'startup_pending'})
        raise RuntimeError('既存のvLLMプロセスが起動待ちです。起動状況とhealthを再確認してから再実行してください。')
    with socket.socket() as probe:
        if probe.connect_ex(('127.0.0.1', PORT)) == 0:
            raise RuntimeError('Port 8000 is already occupied; inspect it without terminating it.')
    log = open('/content/vllm.log', 'a', buffering=1)
    server = subprocess.Popen([
        'vllm', 'serve', MODEL,
        '--max-model-len', str(MAX_LEN),
        '--gpu-memory-utilization', '0.92',
        '--reasoning-parser', 'qwen3',
        '--enable-auto-tool-choice',
        '--served-model-name', SERVED_MODEL,
        '--host', '127.0.0.1', '--port', str(PORT),
        '--api-key', VLLM_API_KEY,
        '--tool-call-parser', 'qwen3_coder',
    ], stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
    print('vLLM process started. Model loading is still pending.')
else:
    print('Reusing the healthy existing LLM server.')

# %% 4. Readiness and authenticated model discovery.
import json
import time
for attempt in range(180):
    try:
        with urllib.request.urlopen('http://127.0.0.1:8000/health', timeout=3) as response:
            if response.status == 200:
                break
    except Exception:
        pass
    if 'server' in globals() and server.poll() is not None:
        raise RuntimeError('vLLM exited. Inspect /content/vllm.log with key redaction before printing.')
    if attempt % 6 == 0:
        print('Waiting for model readiness...', flush=True)
    time.sleep(10)
else:
    raise TimeoutError('LLM readiness timed out; runtime and process were left intact.')
from openai import OpenAI
client = OpenAI(base_url='http://127.0.0.1:8000/v1', api_key=VLLM_API_KEY, timeout=180)
model_ids = [model.id for model in client.models.list().data]
if SERVED_MODEL not in model_ids:
    raise RuntimeError('Expected model ID absent; inspect actual /v1/models before changing configuration.')
print({'models': model_ids, 'local_auth_verified': True})

# %% 5. Reuse a verified tunnel or establish exactly one protected LLM-only tunnel.
import inspect
import ngrok
os.environ['NGROK_AUTHTOKEN'] = NGROK_AUTHTOKEN
# vLLM protects /v1 with the API key, but other generation routes can be unprotected.
# Only these exact method/path pairs may reach the LLM from the public tunnel.
traffic_policy = {
    'on_http_request': [{
        'expressions': [
            "!((req.method == 'GET' && req.url.path == '/v1/models' && req.url.raw_path == '/v1/models') || "
            "(req.method == 'POST' && req.url.path == '/v1/chat/completions' && req.url.raw_path == '/v1/chat/completions'))"
        ],
        'actions': [{'type': 'custom-response', 'config': {
            'status_code': 404, 'body': 'Not found', 'headers': {'content-type': 'text/plain'}
        }}],
    }],
}
policy_json = json.dumps(traffic_policy, sort_keys=True)
existing_tunnel = globals().get('tunnel')
protected_tunnel = (
    existing_tunnel is not None
    and globals().get('_kotonoha_protected_tunnel') is existing_tunnel
    and globals().get('_kotonoha_tunnel_policy') == policy_json
)
if existing_tunnel is not None and not protected_tunnel:
    raise RuntimeError('Existing tunnel policy is unverified. Inspect and explicitly close only that old tunnel, delete the tunnel variable, then rerun this cell.')
listeners = ngrok.get_listeners()
if inspect.isawaitable(listeners):
    listeners = await listeners
if any(not protected_tunnel or listener.url() != existing_tunnel.url() for listener in listeners):
    raise RuntimeError('An unverified ngrok listener is active. Inspect and explicitly close it before creating the protected LLM tunnel.')
if not listeners:
    tunnel = ngrok.forward(
        'http://127.0.0.1:8000', authtoken_from_env=True,
        traffic_policy=policy_json,
    )
    if inspect.isawaitable(tunnel):
        tunnel = await tunnel
    _kotonoha_protected_tunnel = tunnel
    _kotonoha_tunnel_policy = policy_json
PUBLIC_BASE_URL = tunnel.url().rstrip('/') + '/v1'
if not PUBLIC_BASE_URL.startswith('https://'):
    raise RuntimeError('Expected HTTPS tunnel URL.')
# Secret is written only to this private transient file, never notebook output.
connection_path = Path('/content/.kotonoha-llm-connection.json')
connection_fd = os.open(str(connection_path), os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
with os.fdopen(connection_fd, 'w') as connection_file:
    json.dump({'base_url': PUBLIC_BASE_URL, 'model': SERVED_MODEL,
               'api_key': VLLM_API_KEY}, connection_file)
os.chmod(connection_path, 0o600)
print({'base_url': PUBLIC_BASE_URL, 'model': SERVED_MODEL, 'credential_file_ready': True})
