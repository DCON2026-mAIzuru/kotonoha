/**
 * 盆栽チャットシステム - ESP32 ファームウェア
 *
 * 使用センサー:
 *   - DHT22 (温湿度センサー)  : GPIO 4
 *   - 容量型土壌水分センサー   : GPIO 34 (ADC)
 *
 * 動作概要:
 *   - 常時電源（コンセント給電）を前提とした常時稼働ループ
 *   - INTERVAL_MS ごとにセンサー値を取得してバックエンドへ HTTP POST
 *   - 土壌水分が RISE_THRESHOLD % 以上上昇した場合は即座に送信（水やり検知）
 *   - 送信時は X-Sensor-Token / X-Sensor-User-ID ヘッダーを付与
 *   - NTP で取得した ISO 8601 タイムスタンプを recorded_at に付与
 *
 * バックエンド API 仕様 (Step 5 準拠):
 *   POST /api/v1/sensor
 *   Headers: X-Sensor-Token, X-Sensor-User-ID
 *   Body:    { device_id, recorded_at, moisture_percent,
 *              temperature_celsius, light_lux (省略可) }
 *
 * 設定方法:
 *   下記 "===== ユーザー設定 =====" セクションの値を環境に合わせて変更してください。
 */

#include <WiFi.h>
#include <HTTPClient.h>
#include <ArduinoJson.h>
#include <DHT.h>
#include <time.h>

// =====================================================================
// ===== ユーザー設定 =====
// =====================================================================

// --- WiFi 設定 ---
const char* WIFI_SSID      = "YOUR_WIFI_SSID";
const char* WIFI_PASSWORD  = "YOUR_WIFI_PASSWORD";

// --- バックエンド設定 ---
const char* BACKEND_URL    = "https://YOUR_BACKEND_URL/api/v1/sensor";
const char* SENSOR_TOKEN   = "YOUR_SECRET_TOKEN";   // X-Sensor-Token
const char* SENSOR_USER_ID = "YOUR_SUPABASE_USER_ID"; // X-Sensor-User-ID

// --- デバイス識別子 ---
const char* DEVICE_ID      = "esp32-bonsai-001";

// --- 送信間隔 (通常: 10分) ---
const unsigned long INTERVAL_MS = 10UL * 60UL * 1000UL;

// --- 水やり検知しきい値: 前回値から何 % 以上「上昇」したら即時送信するか ---
// Step 5 のイベント発火条件（バックエンド側）は 20% 以上上昇のため、
// ESP32 側は少し低め (15%) で先に送信してバックエンドに判定させる。
const float RISE_THRESHOLD = 15.0f;

// --- ピン設定 ---
#define DHT_PIN         4
#define DHT_TYPE        DHT22
#define MOISTURE_PIN    34   // ADC1_CH6 (入力専用ピン)

// --- NTP 設定 ---
const char* NTP_SERVER  = "pool.ntp.org";
const long  GMT_OFFSET  = 9 * 3600;  // JST (UTC+9)
const int   DST_OFFSET  = 0;

// =====================================================================
// ===== キャリブレーション定数（ESP32 個体に合わせて調整） =====
// =====================================================================
//
// 土壌水分センサーのキャリブレーション
//   MOISTURE_AIR  : センサーを空気中に置いたときの ADC 生値（乾燥 → 0%）
//   MOISTURE_WATER: センサーを水に完全に浸したときの ADC 生値（湿潤 → 100%）
//
//   ※ 容量型センサーは湿るほど ADC 値が下がる傾向があります。
//   ※ Serial Monitor で analogRead(MOISTURE_PIN) の値を読み取り、
//     実測値に合わせてこの 2 値を調整してください。
const int MOISTURE_AIR   = 2800;
const int MOISTURE_WATER = 1200;

// =====================================================================

DHT dht(DHT_PIN, DHT_TYPE);

float lastMoisture = -999.0f;  // 前回送信時の水分値（急変検知用）
unsigned long lastSendMs = 0;  // 前回送信時刻

// -----------------------------------------------------------------
// センサー読み取り・変換
// -----------------------------------------------------------------

/**
 * 土壌水分センサーのアナログ値を 0〜100 % に変換する。
 * 16 回平均でノイズを低減。
 */
float readMoisture() {
  long sum = 0;
  const int SAMPLES = 16;
  for (int i = 0; i < SAMPLES; i++) {
    sum += analogRead(MOISTURE_PIN);
    delay(10);
  }
  int raw = (int)(sum / SAMPLES);

  // ADC 値を 0〜100 % にマッピング（容量型センサーは逆数マッピング）
  float pct = (float)(MOISTURE_AIR - raw)
            / (float)(MOISTURE_AIR - MOISTURE_WATER) * 100.0f;

  // 範囲クランプ
  if (pct < 0.0f)   pct = 0.0f;
  if (pct > 100.0f) pct = 100.0f;

  return pct;
}

/** DHT22 から摂氏温度を読み取る。読み取り失敗時は NaN を返す。 */
float readTemperature() {
  return dht.readTemperature();
}

// -----------------------------------------------------------------
// タイムスタンプ生成
// -----------------------------------------------------------------

/**
 * NTP 同期済みの現在時刻を ISO 8601 形式（JST）で buf へ書き出す。
 * 例: "2026-04-16T10:30:00+09:00"
 */
void getISOTimestamp(char* buf, size_t len) {
  time_t now;
  struct tm timeinfo;
  time(&now);
  localtime_r(&now, &timeinfo);
  // +09:00 固定
  strftime(buf, len, "%Y-%m-%dT%H:%M:%S+09:00", &timeinfo);
}

// -----------------------------------------------------------------
// HTTP POST 送信
// -----------------------------------------------------------------

/**
 * センサーデータを JSON 形式でバックエンドへ POST する。
 * 成功時は HTTP ステータスコード、失敗時は負値を返す。
 *
 * 送信フォーマット (Step 5 準拠):
 *   {
 *     "device_id": "esp32-bonsai-001",
 *     "recorded_at": "2026-04-16T10:30:00+09:00",
 *     "moisture_percent": 62.5,
 *     "temperature_celsius": 24.1
 *   }
 */
int sendSensorData(float moisturePct, float tempCelsius) {
  if (WiFi.status() != WL_CONNECTED) {
    Serial.println("[WARN] WiFi 未接続。再接続を試みます...");
    connectWiFi();
    if (WiFi.status() != WL_CONNECTED) {
      Serial.println("[ERROR] WiFi 再接続失敗。送信をスキップします。");
      return -1;
    }
  }

  HTTPClient http;
  http.begin(BACKEND_URL);
  http.addHeader("Content-Type",    "application/json");
  http.addHeader("X-Sensor-Token",  SENSOR_TOKEN);
  http.addHeader("X-Sensor-User-ID", SENSOR_USER_ID);

  // JSON ペイロード構築
  StaticJsonDocument<256> payload;
  payload["device_id"] = DEVICE_ID;

  char ts[32];
  getISOTimestamp(ts, sizeof(ts));
  payload["recorded_at"] = ts;

  if (!isnan(moisturePct))  payload["moisture_percent"]  = round(moisturePct * 10) / 10.0;
  if (!isnan(tempCelsius))  payload["temperature_celsius"] = round(tempCelsius * 10) / 10.0;

  String body;
  serializeJson(payload, body);

  Serial.print("[INFO] POST -> ");
  Serial.println(body);

  int httpCode = http.POST(body);

  if (httpCode > 0) {
    Serial.printf("[INFO] HTTP Response: %d\n", httpCode);
  } else {
    Serial.printf("[ERROR] HTTP 送信失敗: %s\n", http.errorToString(httpCode).c_str());
  }

  http.end();
  return httpCode;
}

// -----------------------------------------------------------------
// WiFi 接続
// -----------------------------------------------------------------

void connectWiFi() {
  Serial.printf("[INFO] WiFi 接続中: %s\n", WIFI_SSID);

  WiFi.mode(WIFI_STA);
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);

  const unsigned long timeout = 20000;
  unsigned long start = millis();
  while (WiFi.status() != WL_CONNECTED) {
    if (millis() - start > timeout) {
      Serial.println("\n[ERROR] WiFi 接続タイムアウト");
      return;
    }
    delay(500);
    Serial.print(".");
  }

  Serial.println();
  Serial.printf("[INFO] WiFi 接続完了 IP: %s\n", WiFi.localIP().toString().c_str());
}

// -----------------------------------------------------------------
// Arduino エントリポイント
// -----------------------------------------------------------------

void setup() {
  Serial.begin(115200);
  delay(1000);

  Serial.println("===== 盆栽チャットシステム ESP32 起動 =====");

  dht.begin();
  analogReadResolution(12);        // 0〜4095 の 12bit ADC
  analogSetAttenuation(ADC_11db);  // 最大 3.3V 入力レンジ

  connectWiFi();

  // NTP 時刻同期（WiFi 接続後に実行）
  configTime(GMT_OFFSET, DST_OFFSET, NTP_SERVER);
  Serial.print("[INFO] NTP 同期中");
  struct tm timeinfo;
  for (int i = 0; i < 20; i++) {
    if (getLocalTime(&timeinfo)) {
      Serial.println(" 完了");
      break;
    }
    Serial.print(".");
    delay(500);
  }

  // 起動直後に初回送信
  float m = readMoisture();
  float t = readTemperature();
  Serial.printf("[SENSOR] 水分: %.1f%%  温度: %.1f℃\n", m, t);

  int code = sendSensorData(m, t);
  if (code > 0) {
    lastMoisture = m;
    lastSendMs   = millis();
  }
}

void loop() {
  float moisture = readMoisture();
  float temp     = readTemperature();

  Serial.printf("[SENSOR] 水分: %.1f%%  温度: %.1f℃\n", moisture, temp);

  unsigned long now = millis();
  bool intervalElapsed = (now - lastSendMs) >= INTERVAL_MS;

  // 水やり検知: 前回送信値より RISE_THRESHOLD % 以上「上昇」した場合のみ即時送信
  // （乾燥・蒸発による低下は通常の定期送信で対応）
  bool wateringDetected = (lastMoisture >= 0.0f) &&
                          (moisture - lastMoisture >= RISE_THRESHOLD);

  if (intervalElapsed || wateringDetected) {
    if (wateringDetected) {
      Serial.printf("[EVENT] 水分上昇検知: %.1f%% -> %.1f%% (差: +%.1f%%)\n",
                    lastMoisture, moisture, moisture - lastMoisture);
    }

    int code = sendSensorData(moisture, temp);
    if (code > 0) {
      lastMoisture = moisture;
      lastSendMs   = now;
    }
  }

  // 1 秒ごとにループ（センサー監視の解像度）
  delay(1000);
}
