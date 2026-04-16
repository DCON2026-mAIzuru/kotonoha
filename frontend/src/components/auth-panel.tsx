'use client';

import { FormEvent, useState } from 'react';
import { Mail, Lock, Eye, EyeOff, CheckCircle, AlertCircle } from 'lucide-react';

interface AuthPanelProps {
  onSignIn: (email: string, password: string) => Promise<void>;
  onSignUp: (email: string, password: string) => Promise<void>;
  onGoogleSignIn: () => Promise<void>;
}

function GoogleIcon() {
  return (
    <svg className="h-5 w-5" viewBox="0 0 24 24" aria-hidden="true">
      <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" />
      <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" />
      <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" />
      <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" />
    </svg>
  );
}

export function AuthPanel({ onSignIn, onSignUp, onGoogleSignIn }: AuthPanelProps) {
  const [mode, setMode] = useState<'login' | 'register'>('login');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState('');
  const [isError, setIsError] = useState(false);
  const [success, setSuccess] = useState(false);

  const run = async (fn: () => Promise<void>) => {
    setLoading(true);
    setMessage('');
    setIsError(false);
    try {
      await fn();
      setSuccess(true);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '認証に失敗しました');
      setIsError(true);
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (mode === 'register') {
      if (password !== confirmPassword) {
        setMessage('パスワードが一致しません');
        setIsError(true);
        return;
      }
      if (password.length < 6) {
        setMessage('パスワードは6文字以上で設定してください');
        setIsError(true);
        return;
      }
      void run(() => onSignUp(email, password));
    } else {
      void run(() => onSignIn(email, password));
    }
  };

  const switchMode = (next: 'login' | 'register') => {
    setMode(next);
    setMessage('');
    setIsError(false);
    setEmail('');
    setPassword('');
    setConfirmPassword('');
    setSuccess(false);
  };

  return (
    <div className="k-auth-shell">
      <div className="k-auth-card">

        {/* ── ロゴ ── */}
        <div className="k-auth-logo">
          <div className="k-auth-logo-icon">
            <svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M12 19V11M12 11C9.3 11 7 8.9 7 6.2V5h1.8C10.6 5 12 6.4 12 8.2M12 11C14.7 11 17 8.9 17 6.2V5h-1.8C13.4 5 12 6.4 12 8.2" />
            </svg>
          </div>
          <div className="k-auth-logo-text">
            <span>Bonsai Chat</span>
            <span>言ノ葉</span>
          </div>
        </div>

        {/* ── モードタブ ── */}
        <div className="k-auth-tabs">
          <button
            type="button"
            className={`k-auth-tab${mode === 'login' ? ' k-auth-tab-active' : ''}`}
            onClick={() => switchMode('login')}
          >
            ログイン
          </button>
          <button
            type="button"
            className={`k-auth-tab${mode === 'register' ? ' k-auth-tab-active' : ''}`}
            onClick={() => switchMode('register')}
          >
            新規登録
          </button>
        </div>

        {/* ── 成功画面 ── */}
        {success ? (
          <div className="k-auth-success">
            <div className="k-auth-success-icon">
              <CheckCircle size={40} strokeWidth={1.5} />
            </div>
            <p className="k-auth-success-title">
              {mode === 'login' ? 'ログイン成功' : '登録完了'}
            </p>
            <p className="k-auth-success-sub">
              {mode === 'register' ? 'メール確認が必要な場合があります' : '読み込み中...'}
            </p>
          </div>
        ) : (
          <>
            {/* ── エラー ── */}
            {message && isError && (
              <div className="k-auth-error">
                <AlertCircle size={16} strokeWidth={1.8} style={{ flexShrink: 0, marginTop: 1 }} />
                <span>{message}</span>
              </div>
            )}

            {/* ── フォーム ── */}
            <form onSubmit={handleSubmit} style={{ marginTop: 4 }}>
              {/* メール */}
              <div className="k-field">
                <label htmlFor="auth-email">メールアドレス</label>
                <div className="k-input-wrap">
                  <Mail size={16} className="k-input-icon" />
                  <input
                    id="auth-email"
                    type="email"
                    required
                    className="k-input k-input-icon-left"
                    placeholder="you@example.com"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                  />
                </div>
              </div>

              {/* パスワード */}
              <div className="k-field">
                <label htmlFor="auth-password">パスワード</label>
                <div className="k-input-wrap">
                  <Lock size={16} className="k-input-icon" />
                  <input
                    id="auth-password"
                    type={showPassword ? 'text' : 'password'}
                    required
                    minLength={6}
                    className="k-input k-input-icon-left k-input-icon-right"
                    placeholder="6文字以上"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                  />
                  <button
                    type="button"
                    className="k-input-eye"
                    aria-label={showPassword ? 'パスワードを隠す' : 'パスワードを表示'}
                    onClick={() => setShowPassword((v) => !v)}
                  >
                    {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                  </button>
                </div>
              </div>

              {/* 確認パスワード（登録時のみ） */}
              {mode === 'register' && (
                <div className="k-field" style={{ marginBottom: 20 }}>
                  <label htmlFor="auth-confirm">パスワード（確認）</label>
                  <div className="k-input-wrap">
                    <Lock size={16} className="k-input-icon" />
                    <input
                      id="auth-confirm"
                      type={showConfirmPassword ? 'text' : 'password'}
                      required
                      minLength={6}
                      className="k-input k-input-icon-left k-input-icon-right"
                      placeholder="もう一度入力"
                      value={confirmPassword}
                      onChange={(e) => setConfirmPassword(e.target.value)}
                    />
                    <button
                      type="button"
                      className="k-input-eye"
                      aria-label={showConfirmPassword ? 'パスワードを隠す' : 'パスワードを表示'}
                      onClick={() => setShowConfirmPassword((v) => !v)}
                    >
                      {showConfirmPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                    </button>
                  </div>
                </div>
              )}

              <button
                type="submit"
                disabled={loading}
                className="k-btn-primary"
                style={{ marginTop: mode === 'login' ? 20 : 0 }}
              >
                {loading ? '処理中...' : mode === 'login' ? 'ログイン' : '新規登録'}
              </button>
            </form>

            {/* ── 区切り ── */}
            <div className="k-auth-divider">
              <div className="k-auth-divider-line" />
              <span>または</span>
              <div className="k-auth-divider-line" />
            </div>

            {/* ── Google ── */}
            <button
              type="button"
              disabled={loading}
              className="k-btn-google"
              onClick={() => void run(() => onGoogleSignIn())}
            >
              <GoogleIcon />
              Google でログイン
            </button>
          </>
        )}
      </div>
    </div>
  );
}
