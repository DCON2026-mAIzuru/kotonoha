export interface BonsaiProfile {
  user_id: string;
  name: string;
  personality: string;
  backstory: string;
  tone_mode: 'gentle' | 'childlike' | 'calm' | 'classical';
  bond_score: number;
  bond_updated_at: string;
  created_at: string;
  updated_at: string;
}

export interface UpsertProfileInput {
  name: string;
  personality: string;
  backstory: string;
  tone_mode?: 'gentle' | 'childlike' | 'calm' | 'classical';
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

export interface ChatHistoryEntry {
  id: string;
  user_id: string;
  role: 'user' | 'assistant' | 'event' | 'system';
  content: string;
  created_at: string;
}

export interface SensorData {
  user_id: string;
  device_id: string;
  moisture_percent: number;
  light_lux: number;
  temperature_celsius: number;
  recorded_at: string;
  sensor_status: 'normal' | 'stale' | 'invalid' | string;
  created_at: string;
  updated_at: string;
}

export interface MoistureControl {
  paused: boolean;
  manual_value: number | null;
  updated_at?: string;
}

export interface TypingProfile {
  delay_ms: number;
  speed: 'slow' | 'normal' | 'fast';
  bond_level: 'cautious' | 'aware' | 'familiar' | 'attached' | 'trusted';
}

export interface RealtimeMessage {
  type: 'reply_start' | 'reply_chunk' | 'reply_done' | 'error';
  text?: string;
  message?: string;
  source?: string;
  event?: string;
  typing?: TypingProfile;
}
