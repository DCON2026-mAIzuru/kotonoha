'use client';

import { useEffect, useState } from 'react';

interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed'; platform: string }>;
}

function isStandaloneMode(): boolean {
  if (typeof window === 'undefined') {
    return false;
  }

  const navigatorWithStandalone = window.navigator as Navigator & { standalone?: boolean };
  return window.matchMedia('(display-mode: standalone)').matches || navigatorWithStandalone.standalone === true;
}

function isIOSBrowser(): boolean {
  if (typeof window === 'undefined') {
    return false;
  }

  const ua = window.navigator.userAgent.toLowerCase();
  return /iphone|ipad|ipod/.test(ua);
}

interface PwaInstallButtonProps {
  variant?: 'icon' | 'menu';
}

export function PwaInstallButton({ variant = 'icon' }: PwaInstallButtonProps) {
  const [deferredPrompt, setDeferredPrompt] = useState<BeforeInstallPromptEvent | null>(null);
  const [isInstalled, setIsInstalled] = useState(false);
  const [isInstalling, setIsInstalling] = useState(false);
  const [showHelp, setShowHelp] = useState(false);
  const [isIOS, setIsIOS] = useState(false);

  useEffect(() => {
    setIsInstalled(isStandaloneMode());
    setIsIOS(isIOSBrowser());

    const onBeforeInstallPrompt = (event: Event) => {
      event.preventDefault();
      setDeferredPrompt(event as BeforeInstallPromptEvent);
    };

    const onAppInstalled = () => {
      setIsInstalled(true);
      setDeferredPrompt(null);
      setIsInstalling(false);
    };

    const mediaQuery = window.matchMedia('(display-mode: standalone)');
    const onDisplayModeChange = (event: MediaQueryListEvent) => {
      setIsInstalled(event.matches);
      if (event.matches) {
        setDeferredPrompt(null);
      }
    };

    window.addEventListener('beforeinstallprompt', onBeforeInstallPrompt);
    window.addEventListener('appinstalled', onAppInstalled);
    mediaQuery.addEventListener('change', onDisplayModeChange);

    return () => {
      window.removeEventListener('beforeinstallprompt', onBeforeInstallPrompt);
      window.removeEventListener('appinstalled', onAppInstalled);
      mediaQuery.removeEventListener('change', onDisplayModeChange);
    };
  }, []);

  const handleInstall = async () => {
    if (isInstalling) {
      return;
    }

    if (!deferredPrompt) {
      setShowHelp(true);
      return;
    }

    setIsInstalling(true);
    try {
      await deferredPrompt.prompt();
      await deferredPrompt.userChoice;
    } finally {
      setDeferredPrompt(null);
      setIsInstalling(false);
    }
  };

  if (isInstalled) {
    return null;
  }

  const isMenuVariant = variant === 'menu';
  const buttonClassName = isMenuVariant ? 'k-mobile-menu-item' : 'k-status-chip k-status-chip-icon';

  return (
    <>
      <button
        type="button"
        className={buttonClassName}
        onClick={() => void handleInstall()}
        aria-label="言ノ葉をホーム画面に追加"
        title={deferredPrompt ? '言ノ葉をホーム画面に追加' : 'PWAインストール方法を見る'}
      >
        <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
          <path
            d="M12 3v10m0 0 4-4m-4 4-4-4M5 15v2a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-2"
            fill="none"
            stroke="currentColor"
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth="1.8"
          />
        </svg>
        {isMenuVariant && <span>ホーム画面に追加</span>}
      </button>

      {showHelp && (
        <div className="k-install-help-backdrop" onClick={() => setShowHelp(false)}>
          <div className="k-install-help-card" onClick={(event) => event.stopPropagation()}>
            <p className="k-eyebrow" style={{ margin: '0 0 6px' }}>PWA Install</p>
            <h3 style={{ margin: '0 0 8px', fontSize: '1.05rem', fontWeight: 500 }}>ホーム画面に追加</h3>
            <p className="k-install-help-text">
              {isIOS
                ? 'Safari の共有メニューから「ホーム画面に追加」を選ぶと、アプリとして起動できます。'
                : 'このブラウザでは自動インストールがまだ使えません。ブラウザメニューから「インストール」または「ホーム画面に追加」を選んでください。'}
            </p>
            <button type="button" className="k-status-chip" onClick={() => setShowHelp(false)}>
              閉じる
            </button>
          </div>
        </div>
      )}
    </>
  );
}
