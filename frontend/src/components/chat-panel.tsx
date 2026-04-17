'use client';

import { FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { useChatSocket } from '@/hooks/use-chat-socket';

interface ChatPanelProps {
  token: string;
  bonsaiName: string;
  bonsaiSpecies?: string;
  onStatusChange?: (status: 'idle' | 'connecting' | 'open' | 'closed' | 'error') => void;
}

const QUICK_PROMPTS = ['今日の様子を教えて', '水やりは必要？', '光は足りていそう？'] as const;

/* ── SVG icons (ported from bonsAI reference) ── */
function LeafMark() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 19V11M12 11C9.3 11 7 8.9 7 6.2V5h1.8C10.6 5 12 6.4 12 8.2M12 11C14.7 11 17 8.9 17 6.2V5h-1.8C13.4 5 12 6.4 12 8.2" />
    </svg>
  );
}

function WifiMark() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M5 10.5A11 11 0 0 1 19 10.5M8.5 14A6.5 6.5 0 0 1 15.5 14M12 18h.01" />
    </svg>
  );
}

function WaterMark() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 3.8c2.9 3.7 4.8 6.1 4.8 8.6A4.8 4.8 0 1 1 7.2 12.4C7.2 9.9 9.1 7.5 12 3.8Z" />
    </svg>
  );
}

function SunMark() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round">
      <circle cx="12" cy="12" r="3.4" />
      <path d="M12 3.5V6M12 18V20.5M20.5 12H18M6 12H3.5M18 6L16.2 7.8M7.8 16.2L6 18M18 18l-1.8-1.8M7.8 7.8L6 6" />
    </svg>
  );
}

function statusText(status: 'idle' | 'connecting' | 'open' | 'closed' | 'error'): string {
  switch (status) {
    case 'connecting': return '接続中...';
    case 'open':       return 'オンライン';
    case 'closed':     return '切断';
    case 'error':      return 'エラー';
    default:           return '待機中';
  }
}

export function ChatPanel({ token, bonsaiName, bonsaiSpecies, onStatusChange }: ChatPanelProps) {
  const [input, setInput] = useState('');
  const [language, setLanguage] = useState<'ja' | 'en'>('ja');
  const listRef = useRef<HTMLDivElement>(null);

  const { messages, status, errorMessage, wsUrl, sendMessage } = useChatSocket(token, true);
  const isStreaming = useMemo(() => messages.some((m) => m.role === 'assistant' && m.text === ''), [messages]);

  useEffect(() => { onStatusChange?.(status); }, [status, onStatusChange]);

  // Auto-scroll
  useEffect(() => {
    const el = listRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [messages]);

  const handleSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (sendMessage(input)) setInput('');
  };

  const handleQuickPrompt = (prompt: string) => {
    sendMessage(prompt);
  };

  const now = new Date().toLocaleTimeString('ja-JP', { hour: '2-digit', minute: '2-digit', second: '2-digit' });

  return (
    <section className="k-panel">
      <div className="k-chat">

        {/* ── Chat header ── */}
        <header className="k-chat-header">
          <div className="k-chat-title-block">
            <div className="k-avatar" aria-hidden="true">
              <LeafMark />
            </div>
            <div className="k-chat-heading-copy">
              <h2>
                {language === 'ja' ? `${bonsaiName}と話す` : `Talk with ${bonsaiName}`}
              </h2>
              <p className="k-chat-subline">
                {bonsaiSpecies} ・ {now}
              </p>
            </div>
            <span className="k-season-chip">春の気配</span>
          </div>

          <div className="k-chat-header-side">
            {/* Language switcher */}
            <div className="k-lang-switcher">
              <span className={`k-lang-opt${language === 'ja' ? ' k-lang-opt-active' : ''}`}>
                日本語
              </span>
              <button
                type="button"
                className={`k-lang-toggle${language === 'en' ? ' k-lang-toggle-on' : ''}`}
                role="switch"
                aria-checked={language === 'en'}
                aria-label="表示言語を切り替える"
                onClick={() => setLanguage((l) => (l === 'ja' ? 'en' : 'ja'))}
              >
                <span className="k-lang-thumb" />
              </button>
              <span className={`k-lang-opt${language === 'en' ? ' k-lang-opt-active' : ''}`}>
                English
              </span>
            </div>

            {/* Wifi icon */}
            <div className="k-chat-header-signal" aria-hidden="true">
              <WifiMark />
            </div>
          </div>
        </header>

        {/* ── Sensor / status strip ── */}
        <div className="k-sensor-strip">
          <span className="k-mini-badge">
            <WaterMark />
            接続 {statusText(status)}
          </span>
          <span className="k-mini-badge">
            <SunMark />
            会話 {messages.length} 件
          </span>
          <span className="k-mini-badge">
            LLM {wsUrl ? '設定済み' : '未設定'}
          </span>
          <span className="k-mini-badge">
            {language === 'ja' ? '日本語' : 'English'}
          </span>
        </div>

        {/* ── Conversation pane ── */}
        <section className="k-conversation-pane">
          <div ref={listRef} className="k-message-list">
            {messages.length === 0 && (
              <article className="k-message k-message-assistant">
                <div className="k-message-meta">{bonsaiName}</div>
                <p>
                  {language === 'ja'
                    ? `こんにちは〜！今日も元気でぼんすよ！\n根っこでしっかり大地をつかんで、枝もぐんぐん伸びてぼんす。\nうれしいこと、気になること、なんでも話しかけてぼんすねぇ〜！`
                    : `Hey there! Feeling great today!\nRoots gripping strong, branches reaching high — growing every day!\nTell me what's on your mind. I'm all ears (and leaves)!`}
                </p>
              </article>
            )}

            {messages.map((message) => {
              const isUser   = message.role === 'user';
              const isEvent  = message.role === 'event';
              const isSystem = message.role === 'system';

              let cls = 'k-message k-message-assistant';
              if (isUser)   cls = 'k-message k-message-user';
              if (isEvent)  cls = 'k-message k-message-event';
              if (isSystem) cls = 'k-message k-message-system';

              return (
                <article key={message.id} className={cls}>
                  <div className="k-message-meta">
                    {isUser ? 'あなた' : bonsaiName}
                  </div>
                  <p>{message.text || '...'}</p>
                </article>
              );
            })}
          </div>
        </section>

        {/* ── Composer ── */}
        <form className="k-composer" onSubmit={handleSubmit}>
          <div className="k-prompt-row">
            {QUICK_PROMPTS.map((prompt) => (
              <button
                key={prompt}
                type="button"
                className="k-prompt-chip"
                disabled={isStreaming}
                onClick={() => handleQuickPrompt(prompt)}
              >
                {prompt}
              </button>
            ))}
          </div>

          <label className="sr-only" htmlFor="chat-message">メッセージ</label>
          <textarea
            id="chat-message"
            value={input}
            onInput={(e) => setInput((e.target as HTMLTextAreaElement).value)}
            placeholder={
              language === 'ja'
                ? '植物にそっと話しかけるように入力してください'
                : 'Write as if you are gently speaking to your plant'
            }
            rows={4}
            disabled={isStreaming}
          />

          <div className="k-composer-footer">
            <p>
              {isStreaming
                ? `${bonsaiName} が言葉を選んでいます...`
                : (errorMessage || (status === 'open' ? `${wsUrl} に接続中` : `バックエンド: ${wsUrl}`))}
            </p>
            <button
              type="submit"
              className="k-submit-btn"
              disabled={isStreaming || !input.trim()}
            >
              {isStreaming ? 'Streaming...' : (language === 'ja' ? '送信' : 'Send')}
            </button>
          </div>
        </form>

      </div>
    </section>
  );
}
