'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { ChatMessage, RealtimeMessage } from '@/types/api';
import { resolveWebSocketUrl } from '@/lib/ws-url';

const defaultWsUrl = process.env.NEXT_PUBLIC_WS_URL || '';
const defaultApiBaseUrl = process.env.NEXT_PUBLIC_API_BASE_URL || '';

function createId(prefix: string): string {
  return `${prefix}-${Date.now()}-${Math.floor(Math.random() * 100000)}`;
}

function resolveApiBaseUrl(): string {
  const env = defaultApiBaseUrl.trim();
  if (env !== '') {
    return env.replace(/\/$/, '');
  }

  if (typeof window !== 'undefined') {
    return `${window.location.protocol}//${window.location.hostname}:8080`;
  }

  return 'http://localhost:8080';
}

export function useChatSocket(token: string, enabled: boolean) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [status, setStatus] = useState<'idle' | 'connecting' | 'open' | 'closed' | 'error'>('idle');
  const [errorMessage, setErrorMessage] = useState<string>('');
  const wsUrl = useMemo(() => resolveWebSocketUrl(defaultWsUrl), []);
  const apiBaseUrl = useMemo(() => resolveApiBaseUrl(), []);

  const socketRef = useRef<WebSocket | null>(null);
  const activeAssistantIdRef = useRef<string | null>(null);
  const activeEventIdRef = useRef<string | null>(null);

  const appendChunk = useCallback((role: 'assistant' | 'event', chunk: string) => {
    const activeIdRef = role === 'assistant' ? activeAssistantIdRef : activeEventIdRef;

    setMessages((current) => {
      const activeId = activeIdRef.current;
      if (!activeId) {
        const newId = createId(role);
        activeIdRef.current = newId;
        return [...current, { id: newId, role, text: chunk }];
      }

      return current.map((item) => (item.id === activeId ? { ...item, text: item.text + chunk } : item));
    });
  }, []);

  const connect = useCallback(async () => {
    if (!enabled || !token) {
      return;
    }

    setStatus('connecting');
    setErrorMessage('');

    // WebSocket接続前にトークン有効性を確認し、1006の原因を明示する。
    try {
      const authProbe = await fetch(`${apiBaseUrl}/api/v1/auth/me`, {
        headers: {
          Authorization: `Bearer ${token}`,
        },
        cache: 'no-store',
      });

      if (!authProbe.ok) {
        setStatus('error');
        setErrorMessage('認証セッションが無効です。再ログインしてください。');
        return;
      }
    } catch {
      setStatus('error');
      setErrorMessage('認証確認APIに接続できません。バックエンド起動状態を確認してください。');
      return;
    }

    const endpoint = wsUrl.includes('?')
      ? `${wsUrl}&token=${encodeURIComponent(token)}`
      : `${wsUrl}?token=${encodeURIComponent(token)}`;

    const socket = new WebSocket(endpoint);
    socketRef.current = socket;

    socket.onopen = () => {
      setStatus('open');
    };

    socket.onerror = () => {
      setStatus('error');
      setErrorMessage(`WebSocket接続でエラーが発生しました (${wsUrl})`);
    };

    socket.onclose = (event) => {
      setStatus('closed');
      if (event.code !== 1000) {
        if (event.code === 1006) {
          setErrorMessage(
            'WebSocket接続が切断されました (code=1006)。認証切れの可能性があるため再ログインしてください。',
          );
        } else {
          setErrorMessage(`WebSocket接続が切断されました (code=${event.code}, endpoint=${wsUrl})`);
        }
      }
      activeAssistantIdRef.current = null;
      activeEventIdRef.current = null;
    };

    socket.onmessage = (event) => {
      let payload: RealtimeMessage;
      try {
        payload = JSON.parse(event.data) as RealtimeMessage;
      } catch {
        setMessages((current) => [
          ...current,
          { id: createId('system'), role: 'system', text: '受信メッセージの解析に失敗しました' },
        ]);
        return;
      }

      if (payload.type === 'reply_chunk' && payload.text) {
        if (payload.source === 'sensor_event') {
          appendChunk('event', payload.text);
          return;
        }
        appendChunk('assistant', payload.text);
        return;
      }

      if (payload.type === 'reply_done') {
        activeAssistantIdRef.current = null;
        activeEventIdRef.current = null;
        return;
      }

      if (payload.type === 'error') {
        activeAssistantIdRef.current = null;
        activeEventIdRef.current = null;
        // 空のプレースホルダーを除去してからエラーメッセージを追加
        setMessages((current) => [
          ...current.filter((m) => !(m.role === 'assistant' && m.text === '')),
          {
            id: createId('system'),
            role: 'system',
            text: payload.message || '不明なエラーが発生しました',
          },
        ]);
      }
    };
  }, [apiBaseUrl, appendChunk, enabled, token, wsUrl]);

  useEffect(() => {
    connect();

    return () => {
      socketRef.current?.close();
      socketRef.current = null;
    };
  }, [connect]);

  const sendMessage = useCallback((text: string) => {
    const socket = socketRef.current;
    const trimmed = text.trim();
    if (!socket || socket.readyState !== WebSocket.OPEN || trimmed === '') {
      return false;
    }

    // プレースホルダーを先に追加して isStreaming=true にする（LLM の応答待ちを即座にフィードバック）
    const pendingId = createId('assistant');
    activeAssistantIdRef.current = pendingId;
    activeEventIdRef.current = null;

    setMessages((current) => [
      ...current,
      { id: createId('user'), role: 'user', text: trimmed },
      { id: pendingId, role: 'assistant', text: '' },
    ]);

    socket.send(
      JSON.stringify({
        type: 'message',
        text: trimmed,
      }),
    );

    return true;
  }, []);

  const clearMessages = useCallback(() => {
    setMessages([]);
    activeAssistantIdRef.current = null;
    activeEventIdRef.current = null;
  }, []);

  return useMemo(
    () => ({
      messages,
      status,
      errorMessage,
      wsUrl,
      sendMessage,
      clearMessages,
    }),
    [clearMessages, errorMessage, messages, sendMessage, status, wsUrl],
  );
}
