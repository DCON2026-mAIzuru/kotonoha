import type { BonsaiProfile, PushSubscriptionRecord, UpsertProfileInput } from '@/types/api';

const apiBaseUrl = (process.env.NEXT_PUBLIC_API_BASE_URL || 'http://localhost:8080').replace(/\/$/, '');

type JsonBody = Record<string, unknown>;

async function authFetch<T>(token: string, path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${apiBaseUrl}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
      ...(init?.headers || {}),
    },
    cache: 'no-store',
  });

  if (!response.ok) {
    let message = `APIエラー: ${response.status}`;
    try {
      const json = (await response.json()) as { message?: string };
      if (json.message) {
        message = json.message;
      }
    } catch {
      // JSONでないレスポンスはHTTPステータスをそのまま使う。
    }
    throw new Error(message);
  }

  return (await response.json()) as T;
}

export async function getProfile(token: string): Promise<BonsaiProfile | null> {
  const payload = await authFetch<{ profile: BonsaiProfile | null }>(token, '/api/v1/profile');
  return payload.profile;
}

export async function upsertProfile(token: string, input: UpsertProfileInput): Promise<BonsaiProfile> {
  const payload = await authFetch<{ profile: BonsaiProfile }>(token, '/api/v1/profile', {
    method: 'PUT',
    body: JSON.stringify(input),
  });
  return payload.profile;
}

export async function upsertPushSubscription(
  token: string,
  endpoint: string,
  subscription: JsonBody,
): Promise<PushSubscriptionRecord> {
  const payload = await authFetch<{ subscription: PushSubscriptionRecord }>(token, '/api/v1/push/subscribe', {
    method: 'POST',
    body: JSON.stringify({ endpoint, subscription }),
  });

  return payload.subscription;
}
