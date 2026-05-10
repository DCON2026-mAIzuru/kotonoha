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
  raw_payload?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface MoistureControl {
  paused: boolean;
  manual_value: number | null;
  updated_at?: string;
}

export interface SensorIngestResult {
  sensor_data: SensorData;
  event_triggered: boolean;
  event_name?: string;
  event_message?: string;
  push_sent_count?: number;
  notification_warning?: string;
}

export interface PresentationDemoResult {
  initial_reading: SensorData;
  final_result: SensorIngestResult;
  moisture_before: number;
  moisture_after: number;
  scenario: 'moisture_fluctuation' | string;
}

export interface PresentationDemoStatus {
  enabled: boolean;
}

export interface DiagnosisColorRatios {
  green: number;
  yellow: number;
  brown_red: number;
}

export interface BonsaiDiagnosis {
  id: string;
  user_id: string;
  health_score: number;
  status: string;
  status_label: string;
  severity: string;
  severity_label: string;
  detail: string;
  ratios: {
    total: DiagnosisColorRatios;
    top: DiagnosisColorRatios;
    bottom: DiagnosisColorRatios;
  };
  artifact_urls?: Partial<Record<'heatmap' | 'yolo_detection' | 'sam_segmentation' | 'color_extraction', string>>;
  created_at: string;
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
