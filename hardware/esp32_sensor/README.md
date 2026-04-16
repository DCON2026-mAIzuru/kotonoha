# ESP32 センサーファームウェア

## 必要なライブラリ (Arduino IDE ライブラリマネージャーでインストール)

| ライブラリ名 | バージョン目安 |
|---|---|
| `DHT sensor library` (Adafruit) | 1.4.x |
| `ArduinoJson` (Benoit Blanchon) | 7.x |

## 配線

| センサー | ESP32 ピン |
|---|---|
| DHT22 DATA | GPIO 4 |
| 土壌水分センサー OUT (アナログ) | GPIO 34 |
| DHT22 VCC / センサー VCC | 3.3V または 5V (センサー仕様に合わせる) |
| GND | GND |

## 設定手順

`esp32_sensor.ino` の **ユーザー設定** セクションを編集してください。

```cpp
const char* WIFI_SSID     = "YOUR_WIFI_SSID";
const char* WIFI_PASSWORD = "YOUR_WIFI_PASSWORD";
const char* BACKEND_URL   = "https://YOUR_BACKEND_URL/api/sensor";
const char* SENSOR_TOKEN  = "YOUR_SECRET_TOKEN";
```

## キャリブレーション

土壌水分センサーの個体差に合わせて以下の 2 値を調整してください。

1. センサーを **空気中** に置いた状態で `analogRead(MOISTURE_PIN)` の値を読み取り → `MOISTURE_AIR` に設定
2. センサーを **水に完全に浸した** 状態で値を読み取り → `MOISTURE_WATER` に設定

```cpp
const int MOISTURE_AIR   = 2800;  // 実測値に変更
const int MOISTURE_WATER = 1200;  // 実測値に変更
```

## 送信タイミング

- **定期送信**: 10 分ごと (`INTERVAL_MS`)
- **急変検知送信**: 土壌水分が前回送信値から **10% 以上変化** した場合に即時送信 (`CHANGE_THRESHOLD`)

## 送信 JSON 例

```json
{
  "moisture": 62.3,
  "temperature": 22.5,
  "humidity": 58.0
}
```

ヘッダー: `X-Sensor-Token: YOUR_SECRET_TOKEN`
