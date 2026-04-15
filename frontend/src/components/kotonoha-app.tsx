'use client';

import { useEffect, useState } from 'react';
import { AuthPanel } from '@/components/auth-panel';
import { ChatPanel } from '@/components/chat-panel';
import { ProfileSetupForm } from '@/components/profile-setup-form';
import { PushPermissionCard } from '@/components/push-permission-card';
import { useAuth } from '@/hooks/use-auth';
import { getProfile } from '@/lib/backend-api';
import type { BonsaiProfile } from '@/types/api';

export function KotonohaApp() {
  const auth = useAuth();
  const [profile, setProfile] = useState<BonsaiProfile | null>(null);
  const [profileLoading, setProfileLoading] = useState(false);
  const [profileError, setProfileError] = useState('');
  const [llmOnline, setLlmOnline] = useState(false);
  const [showPush, setShowPush] = useState(false);

  useEffect(() => {
    if (!auth.accessToken) {
      setProfile(null);
      return;
    }
    let cancelled = false;
    setProfileLoading(true);
    setProfileError('');
    getProfile(auth.accessToken)
      .then((data) => { if (!cancelled) setProfile(data); })
      .catch((error) => {
        if (!cancelled) setProfileError(error instanceof Error ? error.message : 'プロフィール取得に失敗しました');
      })
      .finally(() => { if (!cancelled) setProfileLoading(false); });
    return () => { cancelled = true; };
  }, [auth.accessToken]);

  /* ── Loading ── */
  if (auth.loading) {
    return (
      <div className="k-shell">
        <div className="k-shell-pattern" />
        <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--muted)', fontSize: '0.9rem' }}>
          認証状態を確認しています...
        </div>
      </div>
    );
  }

  if (auth.configurationError) {
    return (
      <div className="k-shell">
        <div className="k-shell-pattern" />
        <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <div className="k-message-err" style={{ maxWidth: 480 }}>
            Supabase設定が不足しています。環境変数（NEXT_PUBLIC_SUPABASE_URL,
            NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY）を確認してください。
          </div>
        </div>
      </div>
    );
  }

  /* ── Not logged in ── */
  if (!auth.session) {
    return (
      <div className="k-shell">
        <div className="k-shell-pattern" />
        <AuthPanel
          onSignIn={auth.signInWithPassword}
          onSignUp={auth.signUpWithPassword}
          onGoogleSignIn={auth.signInWithGoogle}
        />
      </div>
    );
  }

  if (profileLoading) {
    return (
      <div className="k-shell">
        <div className="k-shell-pattern" />
        <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--muted)', fontSize: '0.9rem' }}>
          プロフィールを読み込んでいます...
        </div>
      </div>
    );
  }

  /* ── Profile setup ── */
  if (!profile) {
    return (
      <div className="k-shell">
        <div className="k-shell-pattern" />
        <div className="k-profile-shell">
          <div style={{ alignSelf: 'flex-end', marginBottom: 12 }}>
            <button type="button" className="k-status-chip" onClick={() => void auth.signOut()}>
              ログアウト
            </button>
          </div>
          <ProfileSetupForm token={auth.accessToken} onSaved={setProfile} />
          {profileError && <p className="k-message-err" style={{ maxWidth: 560 }}>{profileError}</p>}
        </div>
      </div>
    );
  }

  /* ── Main chat view ── */
  return (
    <div className="k-shell">
      <div className="k-shell-pattern" />

      {/* Hero / page header */}
      <header className="k-hero">
        <div className="k-hero-copy">
          <p className="k-eyebrow">Bonsai Chat</p>
          <h1>{profile.name}</h1>
          <p className="k-hero-lead">{profile.personality}</p>
        </div>

        <div className="k-hero-status">
          <span className={`k-status-pill${llmOnline ? ' k-status-pill-online' : ''}`}>
            <span className="k-status-dot" />
            {llmOnline ? 'LLM 接続中' : 'LLM オフライン'}
          </span>

          <div className="k-status-meta">
            <button
              type="button"
              className="k-status-chip"
              onClick={() => setShowPush((v) => !v)}
            >
              通知設定
            </button>
            <button
              type="button"
              className="k-status-chip k-status-chip-green"
              onClick={() => void auth.signOut()}
            >
              ログアウト
            </button>
          </div>
        </div>
      </header>

      {/* Push card (toggled) */}
      {showPush && <PushPermissionCard token={auth.accessToken} />}

      {/* Chat panel – takes the remaining height */}
      <main className="k-chat-layout">
        <ChatPanel
          token={auth.accessToken}
          bonsaiName={profile.name}
          bonsaiSpecies={profile.personality}
          onStatusChange={(status) => setLlmOnline(status === 'open')}
        />
      </main>
    </div>
  );
}
