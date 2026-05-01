/**
 * ESP32 mock sensor sender for Kotonoha
 *
 * Sends mocked moisture / light / temperature values to:
 *   POST /api/v1/sensor
 *
 * Required headers:
 *   X-Sensor-Token
 *   X-Sensor-User-ID
 */

#include <WiFi.h>
#include <WiFiClientSecure.h>
#include <HTTPClient.h>
#include <ArduinoJson.h>
#include <math.h>
#include <time.h>

// ------------------------------------------------------------------
// User settings
// ------------------------------------------------------------------

const char* WIFI_SSID = "****";
const char* WIFI_PASSWORD = "****";

#define BACKEND_HOST "10.172.28.184"
#define BACKEND_URL "http://" BACKEND_HOST ":8080/api/v1/sensor"
const char* SENSOR_TOKEN = "kotonoha-3242c839f8363681a3e1bf9fe5a7497a407738b84546a18b";
const char* SENSOR_USER_ID = "096f1c29-75f1-48a8-ba40-a152b499bb12";
const char* DEVICE_ID = "esp32-demo-001";

const unsigned long SEND_INTERVAL_MS = 10UL * 1000UL;

const char* NTP_SERVER = "pool.ntp.org";
const long GMT_OFFSET_SECONDS = 9 * 3600;
const int DST_OFFSET_SECONDS = 0;

// ------------------------------------------------------------------

unsigned long lastSendMs = 0;
unsigned long readingIndex = 0;

void connectWiFi() {
  Serial.printf("[INFO] Connecting Wi-Fi: %s\n", WIFI_SSID);

  WiFi.mode(WIFI_STA);
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);

  const unsigned long timeoutMs = 20000;
  const unsigned long startedAt = millis();

  while (WiFi.status() != WL_CONNECTED) {
    if (millis() - startedAt > timeoutMs) {
      Serial.println("[ERROR] Wi-Fi connection timed out");
      return;
    }
    delay(500);
    Serial.print(".");
  }

  Serial.println();
  Serial.printf("[INFO] Wi-Fi connected. IP=%s\n", WiFi.localIP().toString().c_str());
}

bool syncTime() {
  configTime(GMT_OFFSET_SECONDS, DST_OFFSET_SECONDS, NTP_SERVER);

  Serial.print("[INFO] Syncing time");
  struct tm timeInfo;
  for (int i = 0; i < 20; i++) {
    if (getLocalTime(&timeInfo)) {
      Serial.println();
      Serial.println("[INFO] Time synced");
      return true;
    }
    Serial.print(".");
    delay(500);
  }

  Serial.println();
  Serial.println("[ERROR] Time sync failed");
  return false;
}

void getIsoTimestamp(char* buffer, size_t length) {
  time_t now;
  struct tm timeInfo;
  time(&now);
  localtime_r(&now, &timeInfo);
  strftime(buffer, length, "%Y-%m-%dT%H:%M:%S+09:00", &timeInfo);
}

float buildMockMoisture(unsigned long index) {
  const float base = 42.0f;
  const int pattern = index % 5;
  return base + (pattern - 2) * 1.7f;
}

float buildMockLight(unsigned long index) {
  const float base = 8500.0f;
  const int pattern = index % 6;
  return base + (pattern - 3) * 320.0f;
}

float buildMockTemperature(unsigned long index) {
  const float base = 24.5f;
  const int pattern = index % 4;
  return base + (pattern - 1) * 0.6f;
}

int sendMockSensorData() {
  if (WiFi.status() != WL_CONNECTED) {
    Serial.println("[WARN] Wi-Fi disconnected. Reconnecting...");
    connectWiFi();
    if (WiFi.status() != WL_CONNECTED) {
      Serial.println("[ERROR] Skip send because Wi-Fi is unavailable");
      return -1;
    }
  }

  const float moisturePercent = buildMockMoisture(readingIndex);
  const float lightLux = buildMockLight(readingIndex);
  const float temperatureCelsius = buildMockTemperature(readingIndex);

  StaticJsonDocument<256> payload;
  payload["device_id"] = DEVICE_ID;

  char timestamp[32];
  getIsoTimestamp(timestamp, sizeof(timestamp));
  payload["recorded_at"] = timestamp;
  payload["moisture_percent"] = roundf(moisturePercent * 10.0f) / 10.0f;
  payload["light_lux"] = roundf(lightLux * 10.0f) / 10.0f;
  payload["temperature_celsius"] = roundf(temperatureCelsius * 10.0f) / 10.0f;

  String body;
  serializeJson(payload, body);

  Serial.print("[INFO] POST ");
  Serial.println(body);

  HTTPClient http;
  String backendUrl = String(BACKEND_URL);
  WiFiClientSecure secureClient;
  if (backendUrl.startsWith("https://")) {
    secureClient.setInsecure();
    http.begin(secureClient, BACKEND_URL);
  } else {
    http.begin(BACKEND_URL);
  }
  http.addHeader("Content-Type", "application/json");
  http.addHeader("X-Sensor-Token", SENSOR_TOKEN);
  http.addHeader("X-Sensor-User-ID", SENSOR_USER_ID);

  const int httpCode = http.POST(body);
  Serial.printf("[INFO] HTTP %d\n", httpCode);

  if (httpCode > 0) {
    const String response = http.getString();
    Serial.print("[INFO] Response ");
    Serial.println(response);
  } else {
    Serial.printf("[ERROR] POST failed: %s\n", http.errorToString(httpCode).c_str());
  }

  http.end();
  readingIndex++;
  return httpCode;
}

void setup() {
  Serial.begin(115200);
  delay(1000);

  Serial.println("===== ESP32 mock sensor sender =====");

  connectWiFi();
  syncTime();

  if (WiFi.status() == WL_CONNECTED) {
    sendMockSensorData();
    lastSendMs = millis();
  }
}

void loop() {
  if (millis() - lastSendMs >= SEND_INTERVAL_MS) {
    sendMockSensorData();
    lastSendMs = millis();
  }

  delay(250);
}
