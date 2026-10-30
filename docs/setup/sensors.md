# センサー送信

## 共通 API

送信先は `POST /api/v1/sensor` です。`X-Sensor-Token` にバックエンドの `SENSOR_SHARED_TOKEN` と同じ値、`X-Sensor-User-ID` に対象の Supabase Auth ユーザー ID を指定します。

```json
{
  "device_id": "esp32-bonsai-001",
  "recorded_at": "2026-04-16T10:30:00+09:00",
  "moisture_percent": 62.5,
  "temperature_celsius": 24.1,
  "light_lux": 8200
}
```

ESP32 からローカルのバックエンドに接続する場合は PC の LAN IP を使います。`localhost` は ESP32 自身を指します。認証情報をリポジトリにコミットしないでください。

## スケッチとシミュレーター

- `esp/esp_mock_sensor/esp_mock_sensor.ino` は水分・照度・温度のモック値を約 10 秒ごとに送ります。Arduino IDE でスケッチを開き、Wi-Fi、送信先 URL、トークン、ユーザー ID を設定して書き込みます。ArduinoJson が必要です。シリアルモニタは 115200 baud で開いて送信結果を確認してください。
- `hardware/esp32_sensor/esp32_sensor.ino` は DHT22 の温度（GPIO 4）と土壌水分（GPIO 34）を送ります。`WIFI_SSID`、`WIFI_PASSWORD`、`BACKEND_URL`、`SENSOR_TOKEN`、`SENSOR_USER_ID` とキャリブレーション値 `MOISTURE_AIR` / `MOISTURE_WATER` を設定します。起動時、10 分ごと、または前回送信値から水分が 15 ポイント以上上昇したときに送信します。湿度は送信しません。ArduinoJson と DHT ライブラリが必要です。
- `esp/kotonohaesp_sensor/kotonohaesp_sensor.ino` は各種センサーとモック値を切り替えるスケッチです。使用ライブラリ、ピン、設定および実際の有効モードはスケッチ先頭を確認してください。
- `hardware/simulator/sensor_simulator.py` は ESP32 を使わない送信確認用です。環境変数 `BACKEND_URL`、`SENSOR_TOKEN`、`SENSOR_USER_ID` を設定し、`python hardware/simulator/sensor_simulator.py --help` で操作を確認してください。

実機の接続やセンサー精度は環境で確認してください。ファームウェアによって送信間隔、センサー種別、通信設定が異なります。

## イベント判定

バックエンドは連続する測定値を比較し、3 分以内に水分が 20 ポイント以上上昇した場合に水やりイベントを判定します。乾燥は 20% 以上から 20% 未満への移行で判定します。同じ種類のイベントには 2 時間のクールダウンがあります。10 分間隔の測定だけでは 3 分以内の水やり条件を満たせないため、実際に送られた時刻と値を確認してください。
