// デモ専用アカウントで認証・保存・WebSocketを検証する。秘密値と生成文は表示しない。
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';
import assert from 'node:assert/strict';
const root = new URL('../../', import.meta.url);
const require = createRequire(new URL('frontend/package.json', root));
const WebSocket = require('ws');
const origin = process.argv.find((arg) => arg.startsWith('--origin='))?.slice(9) || 'http://127.0.0.1:8787';
const expectLLM = process.argv.includes('--expect-llm');
const expectServices = process.argv.includes('--expect-services');
const credentials = JSON.parse(await readFile(new URL('.runtime/supabase-demo-user.json', root), 'utf8'));
const env = Object.fromEntries((await readFile(new URL('backend/.env', root), 'utf8')).split('\n').filter((line) => line.includes('=') && !line.startsWith('#')).map((line) => [line.slice(0, line.indexOf('=')), line.slice(line.indexOf('=') + 1)]));
const results = {};
let token;
async function request(path, options = {}) {
  const response = await fetch(origin + path, { ...options, headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}), ...options.headers }, signal: AbortSignal.timeout(20000) });
  return response;
}
try {
  assert.equal((await request('/readyz')).status, 200);
  assert.equal((await request('/api/v1/profile')).status, 401);
  if (expectServices) {
    assert.ok(expectLLM, '--expect-services requires --expect-llm');
    for (const path of ['/api/v1/tts/voicevox', '/api/v1/chat/memories/search']) {
      assert.equal((await request(path, { method: 'POST', body: JSON.stringify({ text: '確認', query: '確認' }) })).status, 401);
    }
  }
  for (const path of ['/supabase/auth/v1/admin/users', '/supabase/auth/v1/signup', '/supabase/auth/v1/invite']) {
    assert.equal((await request(path)).status, 404);
  }
  assert.ok([401, 403].includes((await request('/supabase/auth/v1/user', { headers: { Authorization: 'Bearer invalid-test-token' } })).status));
  results.access_control = 'passed';
  const login = await request('/supabase/auth/v1/token?grant_type=password', { method: 'POST', body: JSON.stringify({ email: credentials.email, password: credentials.password }) });
  assert.equal(login.status, 200);
  const session = await login.json();
  token = session.access_token;
  assert.equal(session.user.id, credentials.user_id);
  assert.ok(token);
  assert.equal((await request('/supabase/auth/v1/user')).status, 200);
  assert.equal((await request('/api/v1/auth/me')).status, 200);
  results.authentication = 'passed';
  if (expectServices) {
    const voice = await request('/api/v1/tts/voicevox', { method: 'POST', body: JSON.stringify({ text: 'こんにちは。ことの葉です。' }) });
    assert.equal(voice.status, 200);
    assert.ok(voice.headers.get('content-type')?.includes('audio/wav'));
    const wav = Buffer.from(await voice.arrayBuffer());
    assert.ok(wav.length > 44);
    assert.equal(wav.subarray(0, 4).toString(), 'RIFF');
    assert.equal(wav.subarray(8, 12).toString(), 'WAVE');
    results.voicevox = 'authenticated public API returned WAV';
  }
  const profileResponse = await request('/api/v1/profile');
  assert.equal(profileResponse.status, 200);
  const existing = (await profileResponse.json()).profile;
  const profile = existing || { name: 'ことの葉', personality: '穏やかに話す盆栽。短く自然な日本語で返答する。', backstory: '', tone_mode: 'auto' };
  const saved = await request('/api/v1/profile', { method: 'PUT', body: JSON.stringify(profile) });
  assert.equal(saved.status, 200);
  assert.equal((await (await request('/api/v1/profile')).json()).profile.name, profile.name);
  results.profile_saved_and_reloaded = 'passed';
  const previousHistoryResponse = await request('/api/v1/chat/history');
  assert.equal(previousHistoryResponse.status, 200);
  const previousHistoryIDs = new Set((await previousHistoryResponse.json()).history.map((item) => item.id));
  const sent = '[接続確認] 日本語で短く挨拶してください。';
  const wsUrl = new URL('/ws/chat', origin);
  wsUrl.protocol = wsUrl.protocol === 'https:' ? 'wss:' : 'ws:';
  wsUrl.searchParams.set('token', token);
  const types = [];
  let chunks = 0;
  let receivedReply = '';
  await new Promise((resolve, reject) => {
    const socket = new WebSocket(wsUrl, { origin: env.BACKEND_ALLOWED_ORIGIN });
    const timer = setTimeout(() => { socket.terminate(); reject(new Error('WebSocket timeout')); }, 150000);
    const finish = (error) => { clearTimeout(timer); socket.close(); error ? reject(error) : resolve(); };
    socket.on('open', () => socket.send(JSON.stringify({ type: 'message', text: sent })));
    socket.on('error', () => finish(new Error('WebSocket connection failed')));
    socket.on('message', (data) => {
      try {
        const event = JSON.parse(data.toString());
        if (event.source !== 'chat') return;
        types.push(event.type);
        if (event.type === 'reply_chunk' && event.text) {
          chunks += 1;
          receivedReply += event.text;
        }
        if (event.type === 'reply_done') {
          assert.ok(expectLLM && chunks > 0 && receivedReply.trim());
          finish();
        }
        if (event.type === 'error') {
          assert.ok(!expectLLM && event.message.includes('LLMが未接続'));
          assert.equal(chunks, 0);
          finish();
        }
      } catch { finish(new Error('Unexpected WebSocket response')); }
    });
  });
  assert.ok(types.includes('reply_start'));
  results.websocket = expectLLM ? `stream completed (${chunks} chunks)` : 'explicit LLM-disconnected error; no fake response';
  const historyResponse = await request('/api/v1/chat/history');
  assert.equal(historyResponse.status, 200);
  const history = (await historyResponse.json()).history;
  const newHistory = history.filter((item) => !previousHistoryIDs.has(item.id));
  assert.ok(newHistory.some((item) => item.role === 'user' && item.content === sent));
  if (expectLLM) assert.ok(newHistory.some((item) => item.role === 'assistant' && item.content === receivedReply.trim()));
  results.history_persistence = expectLLM ? 'new user message and exact streamed assistant response saved' : 'user message saved; LLM not tested';
  if (expectServices) {
    const search = await request('/api/v1/chat/memories/search', { method: 'POST', body: JSON.stringify({ query: sent, limit: 10 }) });
    assert.equal(search.status, 200);
    const { memories } = await search.json();
    assert.ok(Array.isArray(memories) && memories.length > 0);
    assert.ok(memories.every((memory) => memory.user_id === credentials.user_id));
    assert.ok(memories.some((memory) => memory.summary.includes(sent) && memory.summary.includes(receivedReply.trim())));
    results.semantic_memory = 'new completed conversation embedded, persisted, and retrieved for this user';
  }
  const sensor = await request('/api/v1/sensor');
  assert.equal(sensor.status, 200);
  results.sensor = 'endpoint reachable; physical device not verified';
  results.origin = origin;
  results.real_llm = expectLLM ? 'passed' : 'not verified';
  console.log(JSON.stringify(results, null, 2));
} catch (error) {
  console.error(JSON.stringify({ ...results, verification: 'failed', reason: error instanceof assert.AssertionError ? 'assertion failed' : String(error.message).replace(/https?:\/\/\S+/g, '[URL omitted]') }));
  process.exitCode = 1;
}
