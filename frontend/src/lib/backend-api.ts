import type { BonsaiDiagnosis, BonsaiProfile, ChatHistoryEntry, MoistureControl, PresentationDemoResult, PresentationDemoStatus, PushSubscriptionRecord, SensorData, UpsertProfileInput } from '@/types/api';

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

async function authFormFetch<T>(token: string, path: string, formData: FormData): Promise<T> {
  const response = await fetch(`${apiBaseUrl}${path}`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
    },
    body: formData,
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

async function authBlobFetch(token: string, path: string): Promise<Blob> {
  const response = await fetch(`${apiBaseUrl}${path}`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
    cache: 'no-store',
  });

  if (!response.ok) {
    throw new Error(`画像取得に失敗しました: ${response.status}`);
  }

  return response.blob();
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

export async function getChatHistory(token: string, limit = 50): Promise<ChatHistoryEntry[]> {
  const payload = await authFetch<{ history: ChatHistoryEntry[] }>(token, `/api/v1/chat/history?limit=${limit}`);
  return payload.history ?? [];
}

export async function clearChatHistory(
  token: string,
): Promise<{ deleted_history_count: number; deleted_memory_count: number }> {
  const payload = await authFetch<{
    result: { deleted_history_count: number; deleted_memory_count: number };
  }>(token, '/api/v1/chat/history', {
    method: 'DELETE',
  });
  return payload.result;
}

export async function getCurrentSensorData(token: string): Promise<SensorData | null> {
  const payload = await authFetch<{ sensor_data: SensorData | null }>(token, '/api/v1/sensor');
  return payload.sensor_data;
}

export async function getMoistureControl(token: string): Promise<MoistureControl> {
  const payload = await authFetch<{ moisture_control: MoistureControl }>(token, '/api/v1/sensor/moisture-control');
  return payload.moisture_control;
}

export async function updateMoistureControl(
  token: string,
  input: { paused: boolean; manual_value: number | null },
): Promise<MoistureControl> {
  const payload = await authFetch<{ moisture_control: MoistureControl }>(token, '/api/v1/sensor/moisture-control', {
    method: 'PATCH',
    body: JSON.stringify(input),
  });
  return payload.moisture_control;
}

export async function runPresentationDemo(token: string): Promise<PresentationDemoResult> {
  const payload = await authFetch<{ result: PresentationDemoResult }>(token, '/api/v1/sensor/presentation-demo', {
    method: 'POST',
  });
  return payload.result;
}

export async function getPresentationDemoStatus(token: string): Promise<PresentationDemoStatus> {
  const payload = await authFetch<{ presentation_demo: PresentationDemoStatus }>(token, '/api/v1/sensor/presentation-demo');
  return payload.presentation_demo;
}

export async function uploadDiagnosisImage(token: string, image: File): Promise<BonsaiDiagnosis> {
  const formData = new FormData();
  formData.append('image', image);
  const payload = await authFormFetch<{ diagnosis: BonsaiDiagnosis }>(token, '/api/v1/diagnosis/images', formData);
  return payload.diagnosis;
}

export async function getLatestDiagnosis(token: string): Promise<BonsaiDiagnosis | null> {
  const payload = await authFetch<{ diagnosis: BonsaiDiagnosis | null }>(token, '/api/v1/diagnosis/latest');
  return payload.diagnosis;
}

export async function getDiagnosisArtifact(token: string, path: string): Promise<Blob> {
  return authBlobFetch(token, path);
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

export async function sendDemoPushNotification(
  token: string,
): Promise<{ sent_count: number; failed_count?: number }> {
  const payload = await authFetch<{ result: { sent_count: number; failed_count?: number } }>(token, '/api/v1/push/demo', {
    method: 'POST',
  });

  return payload.result;
}
