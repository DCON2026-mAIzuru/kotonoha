'use client';

import { FormEvent, useState } from 'react';
import { upsertProfile } from '@/lib/backend-api';
import type { BonsaiProfile } from '@/types/api';

interface ProfileSetupFormProps {
  token: string;
  onSaved: (profile: BonsaiProfile) => void;
  onCancel?: () => void;
  initialName?: string;
  initialPersonality?: string;
  initialBackstory?: string;
}

export function ProfileSetupForm({ token, onSaved, onCancel, initialName, initialPersonality, initialBackstory }: ProfileSetupFormProps) {
  const [name, setName] = useState(initialName ?? '言ノ葉');
  const [personality, setPersonality] = useState(initialPersonality ?? '静かで古風だけれど、親しい相手にはやわらかく話す');
  const [backstory, setBackstory] = useState(initialBackstory ?? '前の持ち主は朝に水をくれる人だった。昔は別の庭で育っていた。');
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState('');
  const [isError, setIsError] = useState(false);

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

  return (
    <div className="k-profile-card">
      <p className="k-eyebrow" style={{ margin: '0 0 6px' }}>{onCancel ? 'プロフィール編集' : 'はじめての設定'}</p>
      <h2 style={{ margin: '0 0 4px', fontSize: '1.12rem', fontWeight: 500, letterSpacing: '-0.02em' }}>盆栽のプロフィール</h2>
      <p style={{ margin: '0 0 18px', color: 'var(--muted)', fontSize: '0.82rem' }}>名前と口調、来歴を決めると、返信スタイルに反映されます。</p>

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

      {message && (
        <p className={isError ? 'k-message-err' : 'k-message-ok'}>{message}</p>
      )}
    </div>
  );
}
