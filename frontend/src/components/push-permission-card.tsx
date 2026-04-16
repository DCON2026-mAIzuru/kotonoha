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
  const [isError, setIsError] = useState(false);

  useEffect(() => {
    setSupported(isPushAvailable());
    if (typeof window !== 'undefined' && 'Notification' in window) {
      setPermission(Notification.permission);
    }
  }, []);

  const canEnable = useMemo(
    () => supported && !!vapidPublicKey && permission !== 'denied',
    [permission, supported, vapidPublicKey],
  );

  const enablePush = async () => {
    if (!canEnable) return;
    setLoading(true);
    setStatusText('');
    setIsError(false);
    try {
      const registration = await navigator.serviceWorker.register('/sw.js');
      const result = await Notification.requestPermission();
      setPermission(result);
      if (result !== 'granted') throw new Error('通知許可が拒否されました');

      const existing = await registration.pushManager.getSubscription();
      const subscription =
        existing ||
        (await registration.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: vapidKeyToUint8Array(vapidPublicKey) as BufferSource,
        }));

      const serialized = subscription.toJSON() as Record<string, unknown> & { endpoint?: string };
      await upsertPushSubscription(token, serialized.endpoint || subscription.endpoint, serialized);
      setStatusText('Push通知を有効化しました');
    } catch (error) {
      setStatusText(error instanceof Error ? error.message : 'Push通知設定に失敗しました');
      setIsError(true);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="k-push-card">
      <h3>Push通知</h3>
      <p>水やりイベントを検知したとき、盆栽から自発メッセージを受け取れます。</p>

      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center', marginBottom: 14 }}>
        <span className="k-mini-badge" style={{ fontSize: '0.72rem' }}>
          ブラウザ対応: {supported ? '対応' : '非対応'}
        </span>
        <span className="k-mini-badge" style={{ fontSize: '0.72rem' }}>
          許可: {permission}
        </span>
      </div>

      {!vapidPublicKey && (
        <p className="k-message-err" style={{ marginBottom: 12 }}>
          NEXT_PUBLIC_VAPID_PUBLIC_KEY が未設定です
        </p>
      )}

      <button
        type="button"
        disabled={!canEnable || loading}
        className="k-btn-primary"
        style={{ maxWidth: 200 }}
        onClick={() => void enablePush()}
      >
        {loading ? '設定中...' : '通知を有効化する'}
      </button>

      {statusText && (
        <p className={isError ? 'k-message-err' : 'k-message-ok'} style={{ marginTop: 10 }}>
          {statusText}
        </p>
      )}
    </div>
  );
}
