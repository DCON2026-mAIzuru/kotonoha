# Step 10: ESP32 センサーファームウェア 構築メモ

## 今回追加したもの

- `hardware/simulator/sensor_simulator.py`
  - ESP32 がない環境でデモ・動作確認するための Python シミュレーター
  - 通常モード / 1 回送信モード / 水やりデモモード の 3 種類
  - 標準ライブラリのみで動作（追加インストール不要）
- `hardware/esp32_sensor/esp32_sensor.ino`
  - 常時電源を前提とした常時稼働ループ
  - DHT22 による温度読み取り
  - 容量型土壌水分センサーの ADC 読み取りと 0〜100% への変換（キャリブレーション）
  - NTP による ISO 8601 タイムスタンプ生成
  - 定期送信（10 分ごと）と水やり検知送信（土壌水分 15% 以上上昇で即時送信）
  - `X-Sensor-Token` / `X-Sensor-User-ID` ヘッダー付き HTTP POST

## この段階で変わったこと

- 実機センサーから `POST /api/v1/sensor` へデータを送信できる構成が整いました。
- Step 5 で実装したバックエンドのイベント検知（水分 20% 以上上昇 → Push 通知）が、実際のセンサー値で発火するようになります。
- ESP32 側で 15% 上昇を検知して即時送信 → バックエンド側で 20% 上昇を判定、という 2 段階で水やりを検知します。

## 前提

- Step 2 のバックエンドが起動・デプロイ済みで `POST /api/v1/sensor` が応答できること（Step 8 でデプロイ済み）
- Step 5 の `SENSOR_SHARED_TOKEN` が Fly.io シークレットに登録済みであること
- Arduino IDE または PlatformIO でビルド・書き込みできる環境があること

## 使用センサー・部品

| 部品 | 型番 / 種別 |
|---|---|
| マイコン | ESP32 (ESP-WROOM-32 等) |
| 温湿度センサー | DHT22 (AM2302) または DHT11 |
| 土壌水分センサー | 容量型（抵抗型は腐食するため非推奨） |

## 配線

| センサー側 | ESP32 ピン |
|---|---|
| DHT22 DATA | GPIO 4 |
| 土壌水分センサー OUT (アナログ) | GPIO 34 |
| DHT22 VCC | 3.3V |
| 土壌水分センサー VCC | 3.3V または 5V (センサー仕様に合わせる) |
| GND (全センサー共通) | GND |

> GPIO 34 は ESP32 の入力専用ピンです。analogRead 専用に使用しています。

## 必要なライブラリ

Arduino IDE のライブラリマネージャーから以下をインストールしてください。

| ライブラリ名 | バージョン目安 |
|---|---|
| `DHT sensor library` (Adafruit) | 1.4.x |
| `Adafruit Unified Sensor` (Adafruit) | 1.1.x |
| `ArduinoJson` (Benoit Blanchon) | 7.x |

## 設定手順

`esp32_sensor.ino` の **ユーザー設定** セクション（先頭付近）を環境に合わせて変更します。

```cpp
// WiFi 設定
const char* WIFI_SSID      = "YOUR_WIFI_SSID";
const char* WIFI_PASSWORD  = "YOUR_WIFI_PASSWORD";

// バックエンド設定
const char* BACKEND_URL    = "https://<your-app>.fly.dev/api/v1/sensor";
const char* SENSOR_TOKEN   = "YOUR_SECRET_TOKEN";    // Step 2/.env の SENSOR_SHARED_TOKEN と一致させる
const char* SENSOR_USER_ID = "YOUR_SUPABASE_USER_ID"; // Supabase Auth の user id

// デバイス識別子（複数台管理する場合は個体ごとに変える）
const char* DEVICE_ID      = "esp32-bonsai-001";
```

`SENSOR_TOKEN` は Step 2 の `.env` に設定した `SENSOR_SHARED_TOKEN` と同じ値を使います。  
`SENSOR_USER_ID` は Supabase ダッシュボードの `Authentication > Users` から確認できます。

## キャリブレーション手順

土壌水分センサーの ADC 値は個体差があるため、実測値でキャリブレーション定数を調整します。

```cpp
const int MOISTURE_AIR   = 2800;  // ← 実測値に変更
const int MOISTURE_WATER = 1200;  // ← 実測値に変更
```

### 測定手順

1. Arduino IDE のシリアルモニタを開く（ボーレート: 115200）
2. センサーを **空気中** に置いたまま起動し、シリアル出力の ADC 生値を読み取る  
   → この値を `MOISTURE_AIR` に設定
3. センサー先端を **水に完全に浸し**、ADC 生値を読み取る  
   → この値を `MOISTURE_WATER` に設定
4. 再書き込みして `水分: XX.X%` の表示が 0〜100% の範囲に収まることを確認する

> 容量型センサーは湿るほど ADC 値が下がる傾向があります（`MOISTURE_AIR > MOISTURE_WATER`）。

## 送信タイミング

| トリガー | 条件 | 用途 |
|---|---|---|
| 定期送信 | 10 分ごと (`INTERVAL_MS`) | 通常の状態監視 |
| 水やり検知送信 | 土壌水分が前回送信値から **+15% 以上上昇** | バックエンドのイベント発火を早める |

バックエンド（Step 5）のイベント発火条件は **3 分以内に +20% 以上上昇** です。  
ESP32 側は +15% で先に送信し、バックエンドが 20% の判定を行う 2 段階構成です。

## 送信 JSON 仕様

`POST /api/v1/sensor` へ以下の形式で送信します（Step 5 準拠）。

### リクエストヘッダー

| ヘッダー名 | 値 |
|---|---|
| `Content-Type` | `application/json` |
| `X-Sensor-Token` | `SENSOR_SHARED_TOKEN` と同じ値 |
| `X-Sensor-User-ID` | Supabase の user id |

### リクエストボディ

```json
{
  "device_id": "esp32-bonsai-001",
  "recorded_at": "2026-04-16T10:30:00+09:00",
  "moisture_percent": 62.5,
  "temperature_celsius": 24.1
}
```

- `recorded_at` は NTP 同期した JST (UTC+9) の ISO 8601 形式
- センサー読み取り失敗（NaN）のフィールドは JSON に含めない

## 想定確認手順

1. Arduino IDE でコードを書き込み、シリアルモニタを開く（115200 baud）
2. 起動ログを確認する

```
===== 盆栽チャットシステム ESP32 起動 =====
[INFO] WiFi 接続中: your-ssid
[INFO] WiFi 接続完了 IP: 192.168.x.x
[INFO] NTP 同期中 完了
[SENSOR] 水分: 45.2%  温度: 23.6℃
[INFO] POST -> {"device_id":"esp32-bonsai-001","recorded_at":"...","moisture_percent":45.2,"temperature_celsius":23.6}
[INFO] HTTP Response: 200
```

3. Supabase ダッシュボードの `sensor_data` テーブルに行が追加されることを確認する
4. 土壌水分センサーを水に浸して急上昇を再現し、シリアルモニタに `[EVENT] 水分上昇検知` が出ることを確認する
5. バックエンドのログで `event_triggered: true` が出ること、Push 通知が届くことを確認する

## 現時点の注意

- `SENSOR_USER_ID` は現状ハードコードの暫定運用です（Step 5 の注意事項と同様）
- HTTPS 通信のため、バックエンドが有効な TLS 証明書を持っている必要があります（Fly.io デプロイ済みであれば自動発行済み）
- DHT22 の読み取り間隔は最低 2 秒必要です。`delay(1000)` のループ内で 1 秒ごとに読んでいるため問題ありません
- `GPIO 34` は入力専用ピンのため、プルアップ抵抗不要・書き込み不可です
- WiFi 再接続は起動後の切断に対して自動で試みますが、長時間切断が続く場合はシリアルログを確認してください

## ESP32 なしでデモする方法（センサーシミュレーター）

ESP32 実機がない場合、`hardware/simulator/sensor_simulator.py` で代替できます。  
Python 3 の標準ライブラリのみで動作し、追加インストールは不要です。

### 事前設定

スクリプト先頭の設定セクションを編集するか、環境変数で渡します。

```bash
# 環境変数で渡す場合（推奨）
export BACKEND_URL="https://<your-app>.fly.dev/api/v1/sensor"
export SENSOR_TOKEN="your-secret-token"       # SENSOR_SHARED_TOKEN と同じ値
export SENSOR_USER_ID="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"  # Supabase user id
```

### デモ手順（水やりイベントの発火）

```bash
cd hardware/simulator
python sensor_simulator.py --demo
```

実行すると以下の流れで動きます。

1. 乾燥状態（水分 ~25%）を送信
2. **30 秒待機** ← ここで観客に向けて「水をあげる演技」をする
3. 水やり後（水分 ~70%）を送信 → 差分 +45% でバックエンドのイベントが発火
4. バックエンドが LLM で盆栽のセリフを生成 → WebSocket でフロントへ配信 → Push 通知

期待される出力:

```
盆栽チャットシステム - センサーシミュレーター
  送信先  : https://...fly.dev/api/v1/sensor

[1/3] 乾燥状態を送信します (水分 24.3%)
────────────────────────────────────────────────────────────
  送信時刻  : 2026-04-16T10:30:00+09:00
  水分量    : 24.3 %
  HTTP      : 200
────────────────────────────────────────────────────────────

[2/3] 30 秒後に水やり後の値を送信します。

[3/3] 水やり後の状態を送信します (水分 69.8%)
────────────────────────────────────────────────────────────
  ★ 水やりイベント発火!
  盆栽の反応: お水をもらえて、根の先までほっとしました。
  Push 通知 : 1 件送信
────────────────────────────────────────────────────────────

[DONE] デモ完了。フロントエンドのチャット画面で盆栽の反応を確認してください。
```

### その他のモード

| コマンド | 内容 |
|---|---|
| `python sensor_simulator.py --demo` | 水やりイベントを即時発火（デモ推奨） |
| `python sensor_simulator.py --once` | ランダム値を 1 回だけ送信して終了 |
| `python sensor_simulator.py`        | 10 分ごとに送信し続ける（Ctrl+C で終了） |

### デモ前のチェックリスト

- [ ] バックエンドが Fly.io で稼働していること（`curl https://<app>.fly.dev/healthz` が `200`）
- [ ] フロントエンドを開いてログイン・プロフィール設定済みであること
- [ ] Push 通知の許可を取得済みであること（Step 9 の確認手順 3 番）
- [ ] 過去 2 時間に水やりイベントが発火していないこと（クールダウン中は発火しない）
- [ ] `SENSOR_USER_ID` がフロントエンドにログインしているユーザーと一致していること

## Step 1〜9 との対応関係

| Step | 内容 | ESP32 との関係 |
|---|---|---|
| Step 1 | Supabase DB 構築 | `sensor_data` テーブルにデータが書き込まれる |
| Step 2 | Go/Gin バックエンド基盤 | `X-Sensor-Token` 認証ミドルウェアで受け付ける |
| Step 5 | センサー API / イベント検知 | `POST /api/v1/sensor` がこの ESP32 から呼ばれる |
| Step 8 | Fly.io デプロイ | `BACKEND_URL` に Fly.io のドメインを設定する |
| Step 9 | フロントエンド / PWA | Push 通知・WebSocket がブラウザ側で受け取れる状態 |
