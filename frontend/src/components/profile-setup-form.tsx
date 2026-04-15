'use client';

import { FormEvent, useState } from 'react';
import { upsertProfile } from '@/lib/backend-api';
import type { BonsaiProfile } from '@/types/api';

interface ProfileSetupFormProps {
  token: string;
  onSaved: (profile: BonsaiProfile) => void;
}

export function ProfileSetupForm({ token, onSaved }: ProfileSetupFormProps) {
  const [name, setName] = useState('言ノ葉');
  const [personality, setPersonality] = useState('静かで古風だけれど、親しい相手にはやわらかく話す');
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState('');

  const onSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setLoading(true);
    setMessage('');

    try {
      const profile = await upsertProfile(token, { name, personality });
      onSaved(profile);
    } catch (error) {
      const text = error instanceof Error ? error.message : 'プロフィール保存に失敗しました';
      setMessage(text);
    } finally {
      setLoading(false);
    }
  };

  return (
    <section className="mx-auto w-full max-w-2xl rounded-md border border-white/10 bg-surface-card/95 p-6 shadow-level2">
      <h2 className="text-xl font-bold text-white">最初の設定</h2>
      <p className="mt-2 text-sm text-white/70">盆栽の名前と口調を決めると、返信スタイルに反映されます。</p>

      <form className="mt-6 space-y-4" onSubmit={onSubmit}>
        <div>
          <label className="mb-1 block text-xs text-white/70" htmlFor="profile-name">
            盆栽の名前
          </label>
          <input
            id="profile-name"
            value={name}
            required
            maxLength={40}
            onChange={(event) => setName(event.target.value)}
            className="h-11 w-full rounded border border-white/20 bg-surface-elevated px-3 text-sm text-white outline-none transition focus:border-brand-green"
          />
        </div>

        <div>
          <label className="mb-1 block text-xs text-white/70" htmlFor="profile-personality">
            性格・口調
          </label>
          <textarea
            id="profile-personality"
            value={personality}
            required
            maxLength={300}
            rows={5}
            onChange={(event) => setPersonality(event.target.value)}
            className="w-full rounded border border-white/20 bg-surface-elevated p-3 text-sm text-white outline-none transition focus:border-brand-green"
          />
        </div>

        <button
          type="submit"
          disabled={loading}
          className="h-11 w-full rounded bg-brand-green px-4 text-sm font-bold text-white transition hover:brightness-110 disabled:opacity-50"
        >
          保存してチャットを開始
        </button>
      </form>

      {message && <p className="mt-4 text-xs text-orange-300">{message}</p>}
    </section>
  );
}
