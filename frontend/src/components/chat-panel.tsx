'use client';

import { FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { getCurrentSensorData, getMoistureControl, updateMoistureControl } from '@/lib/backend-api';
import type { MoistureControl, SensorData } from '@/types/api';
import { useChatSocket } from '@/hooks/use-chat-socket';

interface ChatPanelProps {
  token: string;
  bonsaiName: string;
  bonsaiSpecies?: string;
  onStatusChange?: (status: 'idle' | 'connecting' | 'open' | 'closed' | 'error') => void;
  onSensorStateChange?: (state: 'loading' | 'connected' | 'stale' | 'invalid' | 'error' | 'none') => void;
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

function ThermometerMark() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M14 14.8V5a2 2 0 1 0-4 0v9.8a4 4 0 1 0 4 0Z" />
      <path d="M12 11v6" />
    </svg>
  );
}

function ClockMark() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 8v4.5l2.8 1.7" />
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

export function ChatPanel({ token, bonsaiName, bonsaiSpecies, onStatusChange, onSensorStateChange }: ChatPanelProps) {
  const [input, setInput] = useState('');
  const [language, setLanguage] = useState<'ja' | 'en'>('ja');
  const [sensorData, setSensorData] = useState<SensorData | null>(null);
  const [sensorLoading, setSensorLoading] = useState(true);
  const [sensorError, setSensorError] = useState('');
  const [sensorRefreshing, setSensorRefreshing] = useState(false);
  const [moistureControl, setMoistureControl] = useState<MoistureControl | null>(null);
  const [moistureDraft, setMoistureDraft] = useState('');
  const [moistureControlError, setMoistureControlError] = useState('');
  const [moistureControlSaving, setMoistureControlSaving] = useState(false);
  const listRef = useRef<HTMLDivElement>(null);

  const { messages, status, errorMessage, sendMessage } = useChatSocket(token, true);
  const isStreaming = useMemo(() => messages.some((m) => m.role === 'assistant' && m.text === ''), [messages]);

  const loadSensorData = async (options?: { showLoading?: boolean; isManualRefresh?: boolean }) => {
    const showLoading = options?.showLoading ?? false;
    const isManualRefresh = options?.isManualRefresh ?? false;

    if (showLoading) {
      setSensorLoading(true);
    }
    if (isManualRefresh) {
      setSensorRefreshing(true);
    }

    try {
      const [current, control] = await Promise.all([
        getCurrentSensorData(token),
        getMoistureControl(token),
      ]);
      setSensorData(current);
      setMoistureControl(control);
      setSensorError('');
    } catch (error) {
      setSensorError(error instanceof Error ? error.message : 'センサーデータの取得に失敗しました');
    } finally {
      setSensorLoading(false);
      setSensorRefreshing(false);
    }
  };

  useEffect(() => { onStatusChange?.(status); }, [status, onStatusChange]);

  useEffect(() => {
    const nextState = sensorError
      ? 'error'
      : sensorLoading
        ? 'loading'
        : sensorData?.sensor_status === 'normal'
          ? 'connected'
          : sensorData?.sensor_status === 'stale'
            ? 'stale'
            : sensorData?.sensor_status === 'invalid'
              ? 'invalid'
              : 'none';
    onSensorStateChange?.(nextState);
  }, [sensorData, sensorError, sensorLoading, onSensorStateChange]);

  useEffect(() => {
    let cancelled = false;

    const safeLoad = async (showLoading: boolean) => {
      if (showLoading) {
        setSensorLoading(true);
      }
      try {
        const [current, control] = await Promise.all([
          getCurrentSensorData(token),
          getMoistureControl(token),
        ]);
        if (cancelled) {
          return;
        }
        setSensorData(current);
        setMoistureControl(control);
        setSensorError('');
      } catch (error) {
        if (cancelled) {
          return;
        }
        setSensorError(error instanceof Error ? error.message : 'センサーデータの取得に失敗しました');
      } finally {
        if (!cancelled) {
          setSensorLoading(false);
          setSensorRefreshing(false);
        }
      }
    };

    void safeLoad(true);
    const timer = window.setInterval(() => {
      void safeLoad(false);
    }, 30000);

    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [token]);

  useEffect(() => {
    if (moistureControl?.paused) {
      setMoistureDraft(
        typeof moistureControl.manual_value === 'number'
          ? String(Math.round(moistureControl.manual_value))
          : sensorData
            ? String(Math.round(sensorData.moisture_percent))
            : '',
      );
      return;
    }

    setMoistureDraft('');
    setMoistureControlError('');
  }, [moistureControl, sensorData]);

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
  const sensorUpdatedLabel = sensorData
    ? new Date(sensorData.recorded_at).toLocaleString('ja-JP', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
    : '';
  const sensorStatusLabel = sensorData
    ? sensorData.sensor_status === 'invalid'
      ? '一部異常'
      : sensorData.sensor_status === 'stale'
        ? '更新待ち'
        : '正常'
    : sensorLoading
      ? '取得中'
      : sensorError
      ? '取得失敗'
      : '未受信';
  const moisturePaused = moistureControl?.paused ?? false;
  const moistureControlLabel = moisturePaused ? '水分固定中' : '水分自動';
  const backendStatusText = status === 'open'
    ? '盆栽サーバーに接続中'
    : status === 'connecting'
      ? '盆栽サーバーへ接続しています...'
      : `盆栽サーバー: ${statusText(status)}`;

  const handleMoisturePauseToggle = async () => {
    setMoistureControlSaving(true);
    setMoistureControlError('');

    try {
      const nextControl = moisturePaused
        ? await updateMoistureControl(token, { paused: false, manual_value: null })
        : await updateMoistureControl(token, {
            paused: true,
            manual_value: sensorData ? Math.round(sensorData.moisture_percent) : 50,
          });

      setMoistureControl(nextControl);
      await loadSensorData({ isManualRefresh: true });
    } catch (error) {
      setMoistureControlError(error instanceof Error ? error.message : '水分固定の更新に失敗しました');
    } finally {
      setMoistureControlSaving(false);
    }
  };

  const handleMoistureApply = async () => {
    const parsed = Number(moistureDraft);
    if (!Number.isFinite(parsed) || parsed < 0 || parsed > 100) {
      setMoistureControlError('水分値は 0 から 100 の範囲で入力してください');
      return;
    }

    setMoistureControlSaving(true);
    setMoistureControlError('');
    try {
      const nextControl = await updateMoistureControl(token, {
        paused: true,
        manual_value: parsed,
      });
      setMoistureControl(nextControl);
      await loadSensorData({ isManualRefresh: true });
    } catch (error) {
      setMoistureControlError(error instanceof Error ? error.message : '水分値の更新に失敗しました');
    } finally {
      setMoistureControlSaving(false);
    }
  };

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

        <section className="k-sensor-board">
          <div className="k-sensor-board-head">
            <div className="k-sensor-board-summary">
              <span className="k-sensor-chip k-sensor-chip-compact">
                <LeafMark />
                <span className="k-sensor-chip-label">デバイス</span>
                <span className="k-sensor-chip-value">{sensorData?.device_id || '---'}</span>
              </span>
              <span className={`k-sensor-chip k-sensor-chip-compact k-sensor-chip-status-${sensorData?.sensor_status ?? 'unknown'}`}>
                <LeafMark />
                <span className="k-sensor-chip-label">状態</span>
                <span className="k-sensor-chip-value">{sensorStatusLabel}</span>
              </span>
              <span className="k-sensor-chip k-sensor-chip-compact">
                <SunMark />
                <span className="k-sensor-chip-label">照度</span>
                <span className="k-sensor-chip-value">{sensorData ? `${Math.round(sensorData.light_lux)} lux` : '---'}</span>
              </span>
              <span className="k-sensor-chip k-sensor-chip-compact">
                <WaterMark />
                <span className="k-sensor-chip-label">水分</span>
                <span className="k-sensor-chip-value">{sensorData ? `${Math.round(sensorData.moisture_percent)}%` : '---'}</span>
              </span>
              <span className={`k-sensor-chip k-sensor-chip-compact${moisturePaused ? ' k-sensor-chip-status-stale' : ''}`}>
                <WaterMark />
                <span className="k-sensor-chip-label">水分制御</span>
                <span className="k-sensor-chip-value">{moistureControlLabel}</span>
              </span>
              <span className="k-sensor-chip k-sensor-chip-compact">
                <ThermometerMark />
                <span className="k-sensor-chip-label">気温</span>
                <span className="k-sensor-chip-value">{sensorData ? `${sensorData.temperature_celsius.toFixed(1)}℃` : '---'}</span>
              </span>
              <span className="k-sensor-chip k-sensor-chip-compact">
                <ClockMark />
                <span className="k-sensor-chip-label">最終更新</span>
                <span className="k-sensor-chip-value">{sensorUpdatedLabel || '---'}</span>
              </span>
              <span className="k-sensor-chip k-sensor-chip-compact">
                <span className="k-sensor-chip-label">会話</span>
                <span className="k-sensor-chip-value">{messages.length} 件</span>
              </span>
              <span className="k-sensor-chip k-sensor-chip-compact">
                <span className="k-sensor-chip-label">表示</span>
                <span className="k-sensor-chip-value">{language === 'ja' ? '日本語' : 'English'}</span>
              </span>
            </div>
            <div className="k-sensor-board-actions">
              <button
                type="button"
                className="k-esp-refresh-btn"
                onClick={() => void handleMoisturePauseToggle()}
                disabled={sensorLoading || moistureControlSaving}
              >
                {moistureControlSaving
                  ? '更新中...'
                  : moisturePaused
                    ? '水分自動へ戻す'
                    : '水分を一時停止'}
              </button>
              {moisturePaused && (
                <>
                  <label className="k-moisture-input-wrap">
                    <span className="sr-only">固定する水分値</span>
                    <input
                      type="number"
                      inputMode="numeric"
                      min="0"
                      max="100"
                      step="1"
                      value={moistureDraft}
                      onChange={(event) => setMoistureDraft(event.target.value)}
                      disabled={moistureControlSaving}
                      aria-label="固定する水分値"
                    />
                    <span>%</span>
                  </label>
                  <button
                    type="button"
                    className="k-esp-refresh-btn"
                    onClick={() => void handleMoistureApply()}
                    disabled={moistureControlSaving}
                  >
                    値を反映
                  </button>
                </>
              )}
              <button
                type="button"
                className="k-esp-refresh-btn"
                onClick={() => void loadSensorData({ isManualRefresh: true })}
                disabled={sensorLoading || sensorRefreshing || moistureControlSaving}
              >
                {sensorRefreshing ? '再確認中...' : '再確認'}
              </button>
            </div>
          </div>
          {moistureControlError && (
            <p className="k-moisture-control-error">{moistureControlError}</p>
          )}
        </section>

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
              const isPendingAssistant = message.role === 'assistant' && message.text === '';

              let cls = 'k-message k-message-assistant';
              if (isUser)   cls = 'k-message k-message-user';
              if (isEvent)  cls = 'k-message k-message-event';
              if (isSystem) cls = 'k-message k-message-system';
              if (isPendingAssistant) cls += ' k-message-pending';

              return (
                <article key={message.id} className={cls}>
                  <div className="k-message-meta">
                    {isUser ? 'あなた' : bonsaiName}
                  </div>
                  {isPendingAssistant ? (
                    <div className="k-typing-indicator" aria-label={`${bonsaiName} が返答を準備中`}>
                      <span />
                      <span />
                      <span />
                    </div>
                  ) : (
                    <p>{message.text}</p>
                  )}
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
                : (errorMessage || sensorError || backendStatusText)}
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
