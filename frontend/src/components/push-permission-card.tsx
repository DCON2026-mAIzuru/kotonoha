'use client';

import { useEffect, useMemo, useState } from 'react';
import { upsertPushSubscription } from '@/lib/backend-api';
import { isPushAvailable, vapidKeyToUint8Array } from '@/lib/web-push';

interface PushPermissionCardProps {
  token: string;
}

export function PushPermissionCard({ token }: PushPermissionCardProps) {
  const vapidPublicKey = process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY || '';

  const [supported, setSupported] = useState(false);
  const [permission, setPermission] = useState<NotificationPermission>('default');
  const [loading, setLoading] = useState(false);
  const [statusText, setStatusText] = useState('');

  useEffect(() => {
    setSupported(isPushAvailable());
    if (typeof window !== 'undefined' && 'Notification' in window) {
      setPermission(Notification.permission);
    }
  }, []);

  const canEnable = useMemo(() => {
    return supported && !!vapidPublicKey && permission !== 'denied';
  }, [permission, supported, vapidPublicKey]);

  const enablePush = async () => {
    if (!canEnable) {
      return;
    }

    setLoading(true);
    setStatusText('');

    try {
      const registration = await navigator.serviceWorker.register('/sw.js');
      const result = await Notification.requestPermission();
      setPermission(result);

      if (result !== 'granted') {
        throw new Error('通知許可が拒否されました');
      }

      const existing = await registration.pushManager.getSubscription();
      const subscription =
        existing ||
        (await registration.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: vapidKeyToUint8Array(vapidPublicKey) as BufferSource,
        }));

      const serialized = subscription.toJSON() as Record<string, unknown> & { endpoint?: string };
      const endpoint = serialized.endpoint || subscription.endpoint;

      await upsertPushSubscription(token, endpoint, serialized);
      setStatusText('Push通知を有効化しました');
    } catch (error) {
      const text = error instanceof Error ? error.message : 'Push通知設定に失敗しました';
      setStatusText(text);
    } finally {
      setLoading(false);
    }
  };

  return (
    <section className="rounded-md border border-white/10 bg-surface-card/90 p-4">
      <h3 className="text-base font-bold text-white">Push通知</h3>
      <p className="mt-1 text-xs text-white/70">水やりイベントを検知したとき、盆栽から自発メッセージを受け取れます。</p>

      <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-white/70">
        <span>ブラウザ対応: {supported ? '対応' : '非対応'}</span>
        <span>許可状態: {permission}</span>
      </div>

      {!vapidPublicKey && <p className="mt-2 text-xs text-orange-300">NEXT_PUBLIC_VAPID_PUBLIC_KEY が未設定です</p>}

      <button
        type="button"
        disabled={!canEnable || loading}
        onClick={() => {
          void enablePush();
        }}
        className="mt-4 h-10 rounded bg-brand-green px-4 text-sm font-bold text-white transition hover:brightness-110 disabled:opacity-50"
      >
        通知を有効化する
      </button>

      {statusText && <p className="mt-2 text-xs text-white/80">{statusText}</p>}
    </section>
  );
}
