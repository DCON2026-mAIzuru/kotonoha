'use client';

import { FormEvent, useState } from 'react';
import { clearChatHistory, upsertProfile } from '@/lib/backend-api';
import type { BonsaiProfile } from '@/types/api';

interface ProfileSetupFormProps {
  token: string;
  onSaved: (profile: BonsaiProfile) => void;
  onCancel?: () => void;
  initialName?: string;
  initialPersonality?: string;
  initialBackstory?: string;
  initialBondScore?: number;
  onHistoryCleared?: () => void;
}

function bondLabel(score?: number): string {
  const value = typeof score === 'number' ? score : 20;
  if (value >= 80) return '深くなじむ';
  if (value >= 60) return '手になじむ';
  if (value >= 40) return 'なじんできた';
  if (value >= 20) return '覚え始め';
  return 'まだ距離がある';
}

function bondDescription(score?: number): string {
  const value = typeof score === 'number' ? score : 20;
  if (value >= 90) return '親しい癖として、まれに「ぼんす」語尾が出る距離です。';
  if (value >= 80) return '声や手入れのリズムを深く覚えてきています。';
  if (value >= 60) return '日々の世話にかなり慣れて、反応が少し早くなっています。';
  if (value >= 40) return '雑談や水の時間を、日常のものとして受け取り始めています。';
  if (value >= 20) return '声や手入れを少しずつ覚え始めています。';
  return 'まだ様子を見ています。短い声かけと世話で少しずつ近づきます。';
}

export function ProfileSetupForm({
  token,
  onSaved,
  onCancel,
  initialName,
  initialPersonality,
  initialBackstory,
  initialBondScore,
  onHistoryCleared,
}: ProfileSetupFormProps) {
  const [name, setName] = useState(initialName ?? '言ノ葉');
  const [personality, setPersonality] = useState(initialPersonality ?? '静かで古風だけれど、親しい相手にはやわらかく話す');
  const [backstory, setBackstory] = useState(initialBackstory ?? '前の持ち主は朝に水をくれる人だった。昔は別の庭で育っていた。');
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState('');
  const [isError, setIsError] = useState(false);
  const [clearingHistory, setClearingHistory] = useState(false);

  const onSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setLoading(true);
    setMessage('');
    setIsError(false);
    try {
      const profile = await upsertProfile(token, { name, personality, backstory });
      onSaved(profile);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'プロフィール保存に失敗しました');
      setIsError(true);
    } finally {
      setLoading(false);
    }
  };

  const handleClearHistory = async () => {
    const confirmed = window.confirm('会話履歴と長期記憶をすべて削除します。この操作は元に戻せません。削除しますか？');
    if (!confirmed) {
      return;
    }

    setClearingHistory(true);
    setMessage('');
    setIsError(false);
    try {
      const result = await clearChatHistory(token);
      onHistoryCleared?.();
      setMessage(`会話履歴を削除しました（履歴 ${result.deleted_history_count} 件、長期記憶 ${result.deleted_memory_count} 件）。`);
      setIsError(false);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '会話履歴の削除に失敗しました');
      setIsError(true);
    } finally {
      setClearingHistory(false);
    }
  };

  return (
    <div className="k-profile-card">
      <p className="k-eyebrow" style={{ margin: '0 0 6px' }}>{onCancel ? 'プロフィール編集' : 'はじめての設定'}</p>
      <h2 style={{ margin: '0 0 4px', fontSize: '1.12rem', fontWeight: 500, letterSpacing: '-0.02em' }}>盆栽のプロフィール</h2>
      <p style={{ margin: '0 0 18px', color: 'var(--muted)', fontSize: '0.82rem' }}>名前と口調、来歴を決めると、返信スタイルに反映されます。</p>

      {onCancel && (
        <section style={{
          marginBottom: 18,
          padding: '12px 14px',
          borderRadius: 8,
          border: '1px solid rgba(61, 55, 48, 0.14)',
          background: 'rgba(255, 255, 255, 0.52)',
        }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12 }}>
            <div>
              <p style={{ margin: '0 0 2px', color: 'var(--muted)', fontSize: '0.76rem' }}>なじみ具合</p>
              <p style={{ margin: 0, fontSize: '0.98rem', fontWeight: 600 }}>{bondLabel(initialBondScore)}</p>
            </div>
            <span className="k-status-chip" aria-label={`なじみ具合 ${initialBondScore ?? 20}`}>
              {initialBondScore ?? 20}
            </span>
          </div>
          <div
            aria-hidden="true"
            style={{
              height: 6,
              margin: '10px 0 8px',
              borderRadius: 999,
              background: 'rgba(61, 55, 48, 0.14)',
              overflow: 'hidden',
            }}
          >
            <div
              style={{
                width: `${Math.min(Math.max(initialBondScore ?? 20, 0), 100)}%`,
                height: '100%',
                borderRadius: 999,
                background: 'linear-gradient(90deg, #7f9f6b, #d5a94f)',
              }}
            />
          </div>
          <p style={{ margin: 0, color: 'var(--muted)', fontSize: '0.78rem', lineHeight: 1.55 }}>
            {bondDescription(initialBondScore)}
          </p>
        </section>
      )}

      <form onSubmit={onSubmit}>
        <div className="k-field">
          <label htmlFor="profile-name">盆栽の名前</label>
          <input
            id="profile-name"
            type="text"
            required
            maxLength={40}
            className="k-input"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </div>

        <div className="k-field">
          <label htmlFor="profile-personality">性格・口調</label>
          <textarea
            id="profile-personality"
            required
            maxLength={300}
            rows={5}
            className="k-textarea"
            value={personality}
            onChange={(e) => setPersonality(e.target.value)}
          />
        </div>

        <div className="k-field" style={{ marginBottom: 18 }}>
          <label htmlFor="profile-backstory">来歴・昔の思い出</label>
          <textarea
            id="profile-backstory"
            maxLength={2000}
            rows={6}
            className="k-textarea"
            value={backstory}
            onChange={(e) => setBackstory(e.target.value)}
            placeholder="例: 前の持ち主は朝に水をくれる人だった。昔は別の庭で育っていた。"
          />
        </div>

        <div style={{ display: 'flex', gap: 8 }}>
          <button type="submit" disabled={loading} className="k-btn-primary">
            {loading ? '保存中...' : (onCancel ? '保存する' : '保存してチャットを開始')}
          </button>
          {onCancel && (
            <button type="button" disabled={loading} className="k-status-chip" onClick={onCancel}>
              キャンセル
            </button>
          )}
        </div>
      </form>

      {onCancel && (
        <div style={{ marginTop: 20, paddingTop: 16, borderTop: '1px solid rgba(61, 55, 48, 0.14)' }}>
          <p style={{ margin: '0 0 10px', color: 'var(--muted)', fontSize: '0.8rem', lineHeight: 1.6 }}>
            会話履歴を消すと、画面上の会話と長期記憶が削除されます。プロフィールとセンサー状態は残ります。
          </p>
          <button
            type="button"
            disabled={loading || clearingHistory}
            className="k-status-chip"
            onClick={() => void handleClearHistory()}
          >
            {clearingHistory ? '削除中...' : '会話履歴をすべて削除'}
          </button>
        </div>
      )}

      {message && (
        <p className={isError ? 'k-message-err' : 'k-message-ok'}>{message}</p>
      )}
    </div>
  );
}
