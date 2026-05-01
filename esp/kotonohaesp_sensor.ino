/**
 * ESP32 sensor sender for Kotonoha
 *
 * Sends mocked moisture / light / temperature values to:
 *   POST /api/v1/sensor
 *
 * Required headers:
 *   X-Sensor-Token
 *   X-Sensor-User-ID
 */

#include <WiFi.h>
#include <WiFiManager.h>
#include <WiFiClientSecure.h>
#include <HTTPClient.h>
#include <ArduinoJson.h>
#include <math.h>
#include <time.h>
//追加
#include <Wire.h>
#include <Adafruit_Sensor.h>
#include <Adafruit_BME280.h>
#include <BH1750.h>
#include <ModbusMaster.h>
// ------------------------------------------------------------------
// User settings
// ------------------------------------------------------------------
/*
const char* WIFI_SSID = "****";
const char* WIFI_PASSWORD = "****";
*/


#define BACKEND_HOST "10.172.28.184"
#define BACKEND_URL "http://" BACKEND_HOST ":8080/api/v1/sensor"
const char* SENSOR_TOKEN = "kotonoha-3242c839f8363681a3e1bf9fe5a7497a407738b84546a18b";
const char* SENSOR_USER_ID = "096f1c29-75f1-48a8-ba40-a152b499bb12";
const char* DEVICE_ID = "esp32-demo-001";

const unsigned long SEND_INTERVAL_MS = 10UL * 1000UL;

const char* NTP_SERVER = "pool.ntp.org";
const char* NTP_SERVER_FALLBACK = "time.google.com";
const long GMT_OFFSET_SECONDS = 9 * 3600;
const int DST_OFFSET_SECONDS = 0;

// 追加--------------------------------------------------
// --- センサー設定 ---
Adafruit_BME280 bme;
BH1750 lightMeter;
ModbusMaster node;
// RS485用シリアル (HardwareSerial 2)
#define RXD2 16
#define TXD2 17
//ここまで-------------------------------------------------
// ------------------------------------------------------------------

unsigned long lastSendMs = 0;
unsigned long readingIndex = 0;
bool timeSynced = false;

float buildMockMoisture(unsigned long index);
float buildMockTemperature(unsigned long index);
uint16_t buildMockEC(unsigned long index);
float buildMockPh(unsigned long index);

void connectWiFi() {
  Serial.println("[INFO] Connecting Wi-Fi using WiFiManager...");

  WiFiManager wm;

  // 【重要】設定画面のタイムアウト時間を設定（ここでは60秒）
  // 元の20000ms（20秒）の概念を引き継ぎ、設定待ちで永遠に止まるのを防ぎます
  wm.setConfigPortalTimeout(60);

  // "ESP32_Config" としてWi-Fi設定用APを起動、または保存済みのWi-Fiに自動接続
  bool res = wm.autoConnect("ESP32_Config");

  if (!res) {
    Serial.println("[ERROR] Wi-Fi connection failed or timed out");
    // 元のコードと同様にreturnして抜けるか、安全のためにESP.restart();で再起動させます
    // ここでは元の仕様に合わせてreturnにします
    return; 
  }

  // 正常に接続できた場合の処理
  Serial.println();
  Serial.printf("[INFO] Wi-Fi connected. IP=%s\n", WiFi.localIP().toString().c_str());
}

bool syncTime() {
  configTime(GMT_OFFSET_SECONDS, DST_OFFSET_SECONDS, NTP_SERVER, NTP_SERVER_FALLBACK, "ntp.nict.jp");

  Serial.print("[INFO] Syncing time");
  struct tm timeInfo;
  for (int i = 0; i < 40; i++) {
    if (getLocalTime(&timeInfo)) {
      Serial.println();
      Serial.println("[INFO] Time synced");
      timeSynced = true;
      return true;
    }
    Serial.print(".");
    delay(500);
  }

  Serial.println();
  Serial.println("[ERROR] Time sync failed");
  timeSynced = false;
  return false;
}

void getIsoTimestamp(char* buffer, size_t length) {
  time_t now;
  struct tm timeInfo;
  time(&now);
  localtime_r(&now, &timeInfo);
  strftime(buffer, length, "%Y-%m-%dT%H:%M:%S+09:00", &timeInfo);
}

bool isSoilReadingUsable(float moisture, float temperature, uint16_t ec, float ph) {
  if (isnan(moisture) || isnan(temperature) || isnan(ph)) {
    return false;
  }
  if (moisture < 0.0f || moisture > 100.0f) {
    return false;
  }
  if (temperature < -40.0f || temperature > 80.0f) {
    return false;
  }
  if (ph < 3.0f || ph > 10.0f) {
    return false;
  }
  return true;
}

void useMockSoilReading(float* moisture, float* temperature, uint16_t* ec, float* ph) {
  *moisture = buildMockMoisture(readingIndex);
  *temperature = buildMockTemperature(readingIndex);
  *ec = buildMockEC(readingIndex);
  *ph = buildMockPh(readingIndex);
}
//----------------------------ダミー用関数----------------------------------------
/*
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
*/
//-------------------------------------------------------------------------------
// 1. 土壌水分量 (例: 38.6% 〜 45.4%)
float buildMockMoisture(unsigned long index) {
  const float base = 42.0f;
  const int pattern = index % 5;
  return base + (pattern - 2) * 1.7f;
}

// 2. 土壌温度 (例: 23.9℃ 〜 25.7℃)
float buildMockTemperature(unsigned long index) {
  const float base = 24.5f;
  const int pattern = index % 4;
  return base + (pattern - 1) * 0.6f;
}

// 3. 土壌EC (例: 425 〜 575 us/cm)
// ※ECは通常整数値のため、戻り値と変数を uint16_t にしていますが、形式は完全に踏襲しています。
uint16_t buildMockEC(unsigned long index) {
  const uint16_t base = 500;
  const int pattern = index % 7;
  return base + (pattern - 3) * 25;
}

// 4. 土壌pH (例: 6.2 〜 6.8)
float buildMockPh(unsigned long index) {
  const float base = 6.5f;
  const int pattern = index % 4;
  return base + (pattern - 1) * 0.15f;
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

  if (!timeSynced) {
    Serial.println("[WARN] Time is not synced. Retrying NTP before sending...");
    if (!syncTime()) {
      Serial.println("[ERROR] Skip send because time is not synced");
      return -1;
    }
  }
/*--ダミーセンサの値
  const float moisturePercent = buildMockMoisture(readingIndex);
  const float lightLux = buildMockLight(readingIndex);
  const float temperatureCelsius = buildMockTemperature(readingIndex);
*/

float soilMoisturePercent = 0.0;
float soilTemperatureCelsius = 0.0;
uint16_t ecUsCm = 0;
float phValue = 7.0;

//--- 大気（BME280） ---
  const float airHumidityPercent = bme.readHumidity();//湿度
  const float temperatureCelsius = bme.readTemperature();//気温
  const float pressurePascal =bme.readPressure() / 100.0F;//気圧
// --- 照度（BH1750） ---
  const float lightLux = lightMeter.readLightLevel();
// --- 土壌（SEN0604 4-in-1） ---
  // 自動切り替えモジュールなので、シンプルに読み取り要求を出すだけ
  uint8_t result = node.readHoldingRegisters(0x0000, 4);
  


  if (result == node.ku8MBSuccess) {
    soilMoisturePercent = node.getResponseBuffer(0) / 10.0;
    soilTemperatureCelsius = (int16_t)node.getResponseBuffer(1) / 10.0;
    ecUsCm = node.getResponseBuffer(2);
    phValue = node.getResponseBuffer(3) / 10.0;
    Serial.printf("土壌 | raw moisture=%.1f temp=%.1f ec=%u ph=%.1f\n",
                  soilMoisturePercent,
                  soilTemperatureCelsius,
                  ecUsCm,
                  phValue);
    if (!isSoilReadingUsable(soilMoisturePercent, soilTemperatureCelsius, ecUsCm, phValue)) {
      Serial.println("土壌 | 読み取り値が不自然なため動的モックデータを使用します");
      useMockSoilReading(&soilMoisturePercent, &soilTemperatureCelsius, &ecUsCm, &phValue);
    }
  } else {
    Serial.printf("土壌 | 読み取り失敗 (エラーコード: 0x%02X) - 動的モックデータを使用します\n", result);
    useMockSoilReading(&soilMoisturePercent, &soilTemperatureCelsius, &ecUsCm, &phValue);
  } 



  StaticJsonDocument<768> payload;
  payload["device_id"] = DEVICE_ID;

  char timestamp[32];
  getIsoTimestamp(timestamp, sizeof(timestamp));
  payload["recorded_at"] = timestamp;

  payload["moisture_percent"] = roundf(soilMoisturePercent * 10.0f) / 10.0f;//土壌水分量（バックエンド互換）
  payload["air_humidity_percent"] = roundf(airHumidityPercent * 10.0f) / 10.0f;//大気湿度
  payload["light_lux"] = roundf(lightLux * 10.0f) / 10.0f;//照度
  payload["temperature_celsius"] = (roundf(temperatureCelsius * 10.0f) / 10.0f)+(roundf(soilTemperatureCelsius * 10.0f) / 10.0f)/2;//気温
  payload["pressure_pascal"] = roundf(pressurePascal * 10.0f) / 10.0f;//気圧
  payload["soil_moisture_percent"] = roundf(soilMoisturePercent * 10.0f) / 10.0f;//土壌水分量
  payload["soil_ec_us_cm"] = roundf(ecUsCm * 10.0f) / 10.0f;//EC値
  payload["soil_ph_value"] = roundf(phValue * 10.0f) / 10.0f;//ph



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
  http.setTimeout(15000);
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
  //追加-----------------------------------------------------------------
  
  Wire.begin(21, 22); // I2Cの初期化 (SDA=21, SCL=22)

  // 1. BME280 初期化
  if (!bme.begin(0x76, &Wire)) {
    Serial.println("BME280が見つかりません。配線を確認してください。");
  }

  // 2. BH1750 初期化
  if (!lightMeter.begin(BH1750::CONTINUOUS_HIGH_RES_MODE, 0x23, &Wire)) {
    Serial.println("BH1750が見つかりません。");
  }

  // 3. RS485 (SEN0604) 初期化
  Serial2.begin(9600, SERIAL_8N1, RXD2, TXD2);
  node.begin(1, Serial2); // スレーブID 1を設定
  //ここまで---------------------------------------------------------------------
  delay(1000);

  Serial.println("===== ESP32 sensor sender =====");

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
