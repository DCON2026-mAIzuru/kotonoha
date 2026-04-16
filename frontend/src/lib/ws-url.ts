const FALLBACK_WS_PATH = '/ws/chat';

function withWsScheme(value: string): string {
  if (value.startsWith('https://')) {
    return `wss://${value.slice('https://'.length)}`;
  }
  if (value.startsWith('http://')) {
    return `ws://${value.slice('http://'.length)}`;
  }
  return value;
}

function ensurePath(urlString: string): string {
  try {
    const parsed = new URL(urlString);
    if (parsed.pathname === '' || parsed.pathname === '/') {
      parsed.pathname = FALLBACK_WS_PATH;
    }
    return parsed.toString();
  } catch {
    return urlString;
  }
}

export function resolveWebSocketUrl(explicitWsUrl?: string): string {
  const wsEnv = (explicitWsUrl || '').trim();
  if (wsEnv !== '') {
    return ensurePath(withWsScheme(wsEnv));
  }

  const apiBase = (process.env.NEXT_PUBLIC_API_BASE_URL || '').trim();
  if (apiBase !== '') {
    return ensurePath(withWsScheme(apiBase));
  }

  if (typeof window !== 'undefined') {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${protocol}//${window.location.hostname}:8080${FALLBACK_WS_PATH}`;
  }

  return `ws://localhost:8080${FALLBACK_WS_PATH}`;
}
