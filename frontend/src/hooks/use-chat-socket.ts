'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { ChatMessage, RealtimeMessage, TypingProfile } from '@/types/api';
import { getChatHistory } from '@/lib/backend-api';
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
  const historyLoadedRef = useRef(false);
  const assistantCharQueueRef = useRef<string[]>([]);
  const assistantTypingReadyRef = useRef(true);
  const assistantReplyDoneRef = useRef(false);
  const assistantTypingTimerRef = useRef<number | null>(null);
  const assistantTypingIntervalRef = useRef<number | null>(null);
  const typingProfileRef = useRef<TypingProfile | null>(null);

  const appendAssistantText = useCallback((text: string) => {
    setMessages((current) => {
      let activeId = activeAssistantIdRef.current;
      if (!activeId) {
        const pending = [...current].reverse().find((item) => item.role === 'assistant' && item.text === '');
        if (pending) {
          activeAssistantIdRef.current = pending.id;
          activeId = pending.id;
        }
      }
      if (!activeId) {
        const newId = createId('assistant');
        activeAssistantIdRef.current = newId;
        return [...current, { id: newId, role: 'assistant', text }];
      }

      return current.map((item) => (item.id === activeId ? { ...item, text: item.text + text } : item));
    });
  }, []);

  const typingIntervalMs = useCallback(() => {
    switch (typingProfileRef.current?.speed) {
      case 'slow':
        return 52;
      case 'fast':
        return 18;
      default:
        return 32;
    }
  }, []);

  const stopAssistantTypingInterval = useCallback(() => {
    if (assistantTypingIntervalRef.current !== null) {
      window.clearInterval(assistantTypingIntervalRef.current);
      assistantTypingIntervalRef.current = null;
    }
  }, []);

  const finishAssistantIfReady = useCallback(() => {
    if (assistantReplyDoneRef.current && assistantCharQueueRef.current.length === 0) {
      stopAssistantTypingInterval();
      activeAssistantIdRef.current = null;
      assistantReplyDoneRef.current = false;
      typingProfileRef.current = null;
      setMessages((current) => current.filter((message) => !(message.role === 'assistant' && message.text === '')));
    }
  }, [stopAssistantTypingInterval]);

  const startAssistantTypingLoop = useCallback(() => {
    if (!assistantTypingReadyRef.current || assistantTypingIntervalRef.current !== null) {
      return;
    }

    assistantTypingIntervalRef.current = window.setInterval(() => {
      const next = assistantCharQueueRef.current.shift();
      if (next) {
        appendAssistantText(next);
        return;
      }

      finishAssistantIfReady();
    }, typingIntervalMs());
  }, [appendAssistantText, finishAssistantIfReady, typingIntervalMs]);

  const enqueueAssistantChunk = useCallback((chunk: string) => {
    assistantCharQueueRef.current.push(...Array.from(chunk));
    startAssistantTypingLoop();
  }, [startAssistantTypingLoop]);

  const resetAssistantTyping = useCallback(() => {
    if (assistantTypingTimerRef.current !== null) {
      window.clearTimeout(assistantTypingTimerRef.current);
      assistantTypingTimerRef.current = null;
    }
    stopAssistantTypingInterval();
    assistantCharQueueRef.current = [];
    assistantTypingReadyRef.current = true;
    assistantReplyDoneRef.current = false;
    typingProfileRef.current = null;
  }, [stopAssistantTypingInterval]);

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
        enqueueAssistantChunk(payload.text);
        return;
      }

      if (payload.type === 'reply_start') {
        if (payload.source === 'sensor_event') {
          return;
        }
        if (!activeAssistantIdRef.current) {
          setMessages((current) => {
            const pending = [...current].reverse().find((item) => item.role === 'assistant' && item.text === '');
            if (pending) {
              activeAssistantIdRef.current = pending.id;
            }
            return current;
          });
        }
        typingProfileRef.current = payload.typing ?? null;
        assistantTypingReadyRef.current = false;
        assistantReplyDoneRef.current = false;
        if (assistantTypingTimerRef.current !== null) {
          window.clearTimeout(assistantTypingTimerRef.current);
        }
        assistantTypingTimerRef.current = window.setTimeout(() => {
          assistantTypingReadyRef.current = true;
          assistantTypingTimerRef.current = null;
          startAssistantTypingLoop();
        }, Math.max(payload.typing?.delay_ms ?? 0, 0));
        return;
      }

      if (payload.type === 'reply_done') {
        if (payload.source === 'sensor_event') {
          activeEventIdRef.current = null;
          return;
        }
        assistantReplyDoneRef.current = true;
        finishAssistantIfReady();
        activeEventIdRef.current = null;
        return;
      }

      if (payload.type === 'error') {
        resetAssistantTyping();
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
  }, [apiBaseUrl, appendChunk, enabled, enqueueAssistantChunk, finishAssistantIfReady, resetAssistantTyping, startAssistantTypingLoop, token, wsUrl]);

  useEffect(() => {
    if (!enabled || !token || historyLoadedRef.current) return;
    historyLoadedRef.current = true;

    getChatHistory(token).then((entries) => {
      if (entries.length === 0) return;
      setMessages(entries.map((entry) => ({
        id: entry.id,
        role: entry.role,
        text: entry.content,
      })));
    }).catch(() => {
      // 履歴取得失敗時は空のまま続行
    });
  }, [enabled, token]);

  useEffect(() => {
    connect();

    return () => {
      resetAssistantTyping();
      socketRef.current?.close();
      socketRef.current = null;
    };
  }, [connect, resetAssistantTyping]);

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
    resetAssistantTyping();

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
  }, [resetAssistantTyping]);

  const clearMessages = useCallback(() => {
    setMessages([]);
    resetAssistantTyping();
    activeAssistantIdRef.current = null;
    activeEventIdRef.current = null;
  }, [resetAssistantTyping]);

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
