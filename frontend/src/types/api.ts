export interface BonsaiProfile {
  user_id: string;
  name: string;
  personality: string;
  created_at: string;
  updated_at: string;
}

export interface UpsertProfileInput {
  name: string;
  personality: string;
}

export interface PushSubscriptionRecord {
  id: string;
  user_id: string;
  endpoint: string;
  subscription: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant' | 'event' | 'system';
  text: string;
}

export interface RealtimeMessage {
  type: 'reply_chunk' | 'reply_done' | 'error';
  text?: string;
  message?: string;
  source?: string;
  event?: string;
}
