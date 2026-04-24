# Step 10a: ESP32 モックセンサーデータ送信

## 概要

実センサーをまだ接続せず、ESP32 からモック値を `POST /api/v1/sensor` に送る手順です。

この手順で確認できること:

- ESP32 が Wi-Fi に接続できる
- バックエンドのセンサー API に到達できる
- `X-Sensor-Token` / `X-Sensor-User-ID` が正しく通る
- Supabase の `sensor_data` に値が保存される

この段階では以下をすべてモック値で送信します。

- `moisture_percent`
- `light_lux`
- `temperature_celsius`

## 前提

- バックエンドが起動済み、または Fly.io で公開済み
- `SENSOR_SHARED_TOKEN` が設定済み
- Supabase Auth の `user_id` を 1 つ取得済み
- ESP32 を Arduino IDE から書き込める

## 送信先 API

ESP32 は以下へ送信します。

```text
POST /api/v1/sensor
```

必須ヘッダー:

- `Content-Type: application/json`
- `X-Sensor-Token: <SENSOR_SHARED_TOKEN>`
- `X-Sensor-User-ID: <SUPABASE_USER_ID>`

送信 JSON 例:

```json
{
  "device_id": "esp32-mock-001",
  "recorded_at": "2026-04-18T14:30:00+09:00",
  "moisture_percent": 41.2,
  "light_lux": 8200.0,
  "temperature_celsius": 24.6
}
```

## 配置ファイル

この手順用のスケッチ:

- [esp_mock_sensor.ino](/Users/apple/Documents/teamproject/kotonoha/esp/esp_mock_sensor.ino:1)

## 事前に埋める値

`esp/esp_mock_sensor.ino` の先頭で以下を設定します。

```cpp
const char* WIFI_SSID = "YOUR_WIFI_SSID";
const char* WIFI_PASSWORD = "YOUR_WIFI_PASSWORD";
const char* BACKEND_URL = "https://<your-app>.fly.dev/api/v1/sensor";
const char* SENSOR_TOKEN = "YOUR_SENSOR_SHARED_TOKEN";
const char* SENSOR_USER_ID = "YOUR_SUPABASE_USER_ID";
const char* DEVICE_ID = "esp32-mock-001";
```

ローカルのバックエンドへ送る場合は `https://` ではなく、ESP32 から到達できる LAN 上の URL を使ってください。

例:

```text
http://192.168.1.20:8080/api/v1/sensor
```

`localhost` は ESP32 自身を指すため使えません。

## Arduino IDE 設定

1. ESP32 ボード定義を追加する
2. ボードを自分の ESP32 に合わせて選ぶ
3. 必要ライブラリを入れる

必要ライブラリ:

- `ArduinoJson`

`WiFi.h` と `HTTPClient.h` は ESP32 ボードパッケージ側に含まれます。

## 書き込み手順

1. Arduino IDE で [esp_mock_sensor.ino](/Users/apple/Documents/teamproject/kotonoha/esp/esp_mock_sensor.ino:1) を開く
2. Wi-Fi / API 設定を埋める
3. ESP32 を USB 接続する
4. ボードとポートを選択する
5. 書き込む
6. シリアルモニタを `115200 baud` で開く

## 動作

このスケッチは以下を行います。

- 起動時に Wi-Fi 接続
- NTP で JST 時刻を取得
- 10 秒ごとにモック値を生成
- JSON をバックエンドへ POST
- レスポンスコードとレスポンス本文をシリアルに表示

モック値は完全固定ではなく、少しだけ揺らぎます。
そのため、保存結果が毎回わずかに変わります。

## シリアルモニタ期待値

```text
===== ESP32 mock sensor sender =====
[INFO] Connecting Wi-Fi: your-ssid
[INFO] Wi-Fi connected. IP=192.168.1.15
[INFO] Time synced
[INFO] POST {"device_id":"esp32-mock-001","recorded_at":"2026-04-18T14:30:00+09:00","moisture_percent":41.2,"light_lux":8200,"temperature_celsius":24.6}
[INFO] HTTP 200
[INFO] Response {"sensor_data": ... }
```

## 確認手順

1. `curl https://<backend>/healthz` が `200` で返ることを確認
2. ESP32 のシリアルモニタで `HTTP 200` を確認
3. フロントエンドを開いた状態でチャット画面を確認
4. Supabase の `sensor_data` テーブルで最新値が更新されることを確認

## 失敗時の見方

- `401`
  `X-Sensor-Token` が `SENSOR_SHARED_TOKEN` と一致していません
- `400`
  `recorded_at` や JSON 形式が不正です
- `404`
  URL が `/api/v1/sensor` になっていません
- 接続失敗
  ESP32 からその URL に到達できていません。ローカルなら PC の LAN IP を使ってください

## 次の段階

モック送信で通信確認ができたら、次に進むとよいです。

1. `light_lux` だけ実センサー値に置き換える
2. `moisture_percent` を実センサーに置き換える
3. `temperature_celsius` を実センサーに置き換える

最初は 1 個ずつ置き換える方が切り分けしやすいです。
