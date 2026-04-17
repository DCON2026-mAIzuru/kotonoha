'use client';

import { useEffect, useState } from 'react';
import { AuthPanel } from '@/components/auth-panel';
import { ChatPanel } from '@/components/chat-panel';
import { PwaInstallButton } from '@/components/pwa-install-button';
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
  const [showEditProfile, setShowEditProfile] = useState(false);
  const [showMobileMenu, setShowMobileMenu] = useState(false);

  useEffect(() => {
    if (!showMobileMenu) {
      return;
    }

    const previousOverflow = document.body.style.overflow;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setShowMobileMenu(false);
      }
    };

    document.body.style.overflow = 'hidden';
    window.addEventListener('keydown', onKeyDown);

    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener('keydown', onKeyDown);
    };
  }, [showMobileMenu]);

  useEffect(() => {
    const mediaQuery = window.matchMedia('(min-width: 981px)');
    const onViewportChange = (event: MediaQueryListEvent) => {
      if (event.matches) {
        setShowMobileMenu(false);
      }
    };

    mediaQuery.addEventListener('change', onViewportChange);
    return () => mediaQuery.removeEventListener('change', onViewportChange);
  }, []);

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

  const closeMobileMenu = () => setShowMobileMenu(false);

  const openEditProfile = () => {
    setShowEditProfile(true);
    setShowPush(false);
    closeMobileMenu();
  };

  const openPushSettings = () => {
    setShowPush(true);
    setShowEditProfile(false);
    closeMobileMenu();
  };

  const closeEditProfile = () => setShowEditProfile(false);
  const closePushSettings = () => setShowPush(false);

  const signOut = async () => {
    closeMobileMenu();
    await auth.signOut();
  };

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
            <PwaInstallButton />
            <button
              type="button"
              className="k-status-chip"
              onClick={openEditProfile}
            >
              プロフィール編集
            </button>
            <button
              type="button"
              className="k-status-chip"
              onClick={openPushSettings}
            >
              通知設定
            </button>
            <button
              type="button"
              className="k-status-chip k-status-chip-green"
              onClick={() => void signOut()}
            >
              ログアウト
            </button>
          </div>

          <button
            type="button"
            className="k-mobile-menu-toggle"
            onClick={() => setShowMobileMenu((value) => !value)}
            aria-label={showMobileMenu ? 'メニューを閉じる' : 'メニューを開く'}
            aria-expanded={showMobileMenu}
            aria-controls="mobile-menu"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
              <path
                d="M4 7h16M4 12h16M4 17h16"
                fill="none"
                stroke="currentColor"
                strokeLinecap="round"
                strokeWidth="1.8"
              />
            </svg>
          </button>
        </div>
      </header>

      {showMobileMenu && (
        <div className="k-mobile-menu-backdrop" onClick={closeMobileMenu}>
          <div
            id="mobile-menu"
            className="k-mobile-menu"
            role="dialog"
            aria-modal="true"
            aria-label="モバイルメニュー"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="k-mobile-menu-header">
              <p className="k-mobile-menu-title">メニュー</p>
              <button
                type="button"
                className="k-mobile-menu-close"
                onClick={closeMobileMenu}
                aria-label="メニューを閉じる"
              >
                <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
                  <path
                    d="M7 7l10 10M17 7 7 17"
                    fill="none"
                    stroke="currentColor"
                    strokeLinecap="round"
                    strokeWidth="1.8"
                  />
                </svg>
              </button>
            </div>
            <div className="k-mobile-menu-actions">
              <PwaInstallButton variant="menu" onPress={closeMobileMenu} />
              <button type="button" className="k-mobile-menu-item" onClick={openEditProfile}>
                プロフィール編集
              </button>
              <button type="button" className="k-mobile-menu-item" onClick={openPushSettings}>
                通知設定
              </button>
              <button type="button" className="k-mobile-menu-item k-mobile-menu-item-green" onClick={() => void signOut()}>
                ログアウト
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Profile edit modal (toggled) */}
      {showEditProfile && (
        <div
          style={{
            position: 'fixed', inset: 0, zIndex: 100,
            background: 'rgba(42, 41, 37, 0.46)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            padding: '24px 16px',
          }}
          onClick={closeEditProfile}
        >
          <div onClick={(e) => e.stopPropagation()} style={{ width: '100%', maxWidth: 560 }}>
            <ProfileSetupForm
              token={auth.accessToken}
              initialName={profile.name}
              initialPersonality={profile.personality}
              onSaved={(updated) => { setProfile(updated); closeEditProfile(); }}
              onCancel={closeEditProfile}
            />
          </div>
        </div>
      )}

      {/* Push modal (toggled) */}
      {showPush && (
        <div
          style={{
            position: 'fixed', inset: 0, zIndex: 100,
            background: 'rgba(42, 41, 37, 0.46)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            padding: '24px 16px',
          }}
          onClick={closePushSettings}
        >
          <div onClick={(e) => e.stopPropagation()} style={{ width: '100%', maxWidth: 480 }}>
            <PushPermissionCard token={auth.accessToken} onClose={closePushSettings} />
          </div>
        </div>
      )}

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
