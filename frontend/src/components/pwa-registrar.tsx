'use client';

import { useEffect } from 'react';

export function PwaRegistrar() {
  useEffect(() => {
    if (typeof window === 'undefined' || !('serviceWorker' in navigator)) {
      return;
    }

    navigator.serviceWorker.register('/sw.js').catch(() => {
      // Service Worker登録の失敗はアプリ全体を止めない。
    });
  }, []);

  return null;
}
