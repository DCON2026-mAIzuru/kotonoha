'use client';

import { FormEvent, useState } from 'react';

interface AuthPanelProps {
  onSignIn: (email: string, password: string) => Promise<void>;
  onSignUp: (email: string, password: string) => Promise<void>;
  onGoogleSignIn: () => Promise<void>;
}

export function AuthPanel({ onSignIn, onSignUp, onGoogleSignIn }: AuthPanelProps) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState('');

  const handleSubmit = async (event: FormEvent<HTMLFormElement>, mode: 'signin' | 'signup') => {
    event.preventDefault();
    setLoading(true);
    setMessage('');
    try {
      if (mode === 'signin') {
        await onSignIn(email, password);
      } else {
        await onSignUp(email, password);
        setMessage('サインアップが完了しました。メール確認が必要な場合があります。');
      }
    } catch (error) {
      const text = error instanceof Error ? error.message : '認証に失敗しました';
      setMessage(text);
    } finally {
      setLoading(false);
    }
  };

  const handleGoogle = async () => {
    setLoading(true);
    setMessage('');
    try {
      await onGoogleSignIn();
    } catch (error) {
      const text = error instanceof Error ? error.message : 'Googleログインに失敗しました';
      setMessage(text);
      setLoading(false);
    }
  };

  const handleSignUp = async () => {
    setLoading(true);
    setMessage('');
    try {
      await onSignUp(email, password);
      setMessage('サインアップが完了しました。メール確認が必要な場合があります。');
    } catch (error) {
      const text = error instanceof Error ? error.message : '新規登録に失敗しました';
      setMessage(text);
    } finally {
      setLoading(false);
    }
  };

  return (
    <section className="mx-auto w-full max-w-xl rounded-md border border-white/10 bg-surface-card/95 p-6 shadow-level2 backdrop-blur">
      <h1 className="text-2xl font-bold text-white">言ノ葉にログイン</h1>
      <p className="mt-2 text-sm text-white/70">Supabase Auth でログインして、盆栽との会話をはじめます。</p>

      <form className="mt-6 space-y-4" onSubmit={(event) => handleSubmit(event, 'signin')}>
        <div>
          <label className="mb-1 block text-xs text-white/70" htmlFor="email">
            メールアドレス
          </label>
          <input
            id="email"
            type="email"
            required
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            className="h-11 w-full rounded border border-white/20 bg-surface-elevated px-3 text-sm text-white outline-none transition focus:border-brand-green"
            placeholder="you@example.com"
          />
        </div>

        <div>
          <label className="mb-1 block text-xs text-white/70" htmlFor="password">
            パスワード
          </label>
          <input
            id="password"
            type="password"
            required
            minLength={6}
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            className="h-11 w-full rounded border border-white/20 bg-surface-elevated px-3 text-sm text-white outline-none transition focus:border-brand-green"
            placeholder="6文字以上"
          />
        </div>

        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
          <button
            type="submit"
            disabled={loading}
            className="h-11 rounded bg-brand-green px-4 text-sm font-bold text-white transition hover:brightness-110 disabled:opacity-50"
          >
            ログイン
          </button>
          <button
            type="button"
            disabled={loading}
            onClick={() => {
              void handleSignUp();
            }}
            className="h-11 rounded border border-white/30 bg-transparent px-4 text-sm font-bold text-white transition hover:border-white/60 disabled:opacity-50"
          >
            新規登録
          </button>
        </div>
      </form>

      <div className="mt-4 border-t border-white/10 pt-4">
        <button
          type="button"
          disabled={loading}
          onClick={() => {
            void handleGoogle();
          }}
          className="h-11 w-full rounded bg-brand-pink px-4 text-sm font-bold text-white transition hover:brightness-110 disabled:opacity-50"
        >
          Googleでログイン
        </button>
      </div>

      {message && <p className="mt-4 text-xs text-white/80">{message}</p>}
    </section>
  );
}
