# Step 5: センサー連携APIとイベント検知・Push通知 構築メモ

## 今回追加したもの

- `POST /api/v1/sensor`
  センサー値を受け取り、最新状態を `sensor_data` に保存する API
- `SensorService`
  センサー入力の検証、最新値更新、水分急上昇イベント判定、通知文生成を担当
- `internal/realtime`
  ユーザーごとの WebSocket 接続を保持し、サーバー起点のメッセージを配信するハブ
- `internal/webpush`
  VAPID を使って Push Subscription へ通知を送る送信処理
- `chat_history` への `role=event` 保存
  自発通知を履歴として残し、2時間クールダウン判定にも利用

## 追加されたエンドポイント

- `POST /api/v1/sensor`
  固定トークン付きでセンサー値を受け取り、必要に応じて自発通知を発火する

## リクエスト仕様

### 必須ヘッダー

- `X-Sensor-Token`
  `.env` の `SENSOR_SHARED_TOKEN`
- `X-Sensor-User-ID`
  通知先ユーザーの Supabase user id

### JSON ボディ

```json
{
  "device_id": "mock-device-001",
  "recorded_at": "2026-04-14T10:30:00+09:00",
  "moisture_percent": 62.5,
  "light_lux": 9800,
  "temperature_celsius": 24.1
}
```

## イベント発火条件

- 直前の保存値から `3分以内`
- `moisture_percent` が `20` ポイント以上上昇
- 直近 `2時間` に `role=event` の自発通知がない

条件を満たした場合、LLM で通知文を生成し、以下を同時に行います。

- アクティブな WebSocket 接続へ `reply_chunk` / `reply_done` を送信
- 登録済み Push Subscription へ Web Push を送信
- `chat_history` に `role=event` で保存

## WebSocket 追加フィールド

通常の `reply_chunk` / `reply_done` に加えて、センサー起点の通知では以下のフィールドを付けています。

```json
{
  "type": "reply_chunk",
  "text": "お水をもらえて、ほっとしました。",
  "source": "sensor_event",
  "event": "watering_detected"
}
```

## レスポンス例

```json
{
  "sensor_data": {
    "user_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
    "device_id": "mock-device-001",
    "moisture_percent": 62.5,
    "light_lux": 9800,
    "temperature_celsius": 24.1,
    "recorded_at": "2026-04-14T01:30:00Z",
    "sensor_status": "normal",
    "created_at": "2026-04-14T01:30:00Z",
    "updated_at": "2026-04-14T01:30:00Z"
  },
  "event_triggered": true,
  "event_name": "watering_detected",
  "event_message": "お水をもらえて、葉の先までほっとしました。ありがとう。",
  "push_sent_count": 1
}
```

## 現時点の注意

- `VAPID_PUBLIC_KEY` / `VAPID_PRIVATE_KEY` / `VAPID_SUBJECT` が未設定の場合、Push送信はスキップされる
- センサー送信先ユーザーの解決は、現段階では `X-Sensor-User-ID` ヘッダーで受ける暫定運用
- `sensor_data` は最新値のみ保持し、急上昇判定は「直前の1件」と比較している
- 自発通知の文章生成にはプロフィール名・性格を反映するが、通常チャット側の動的プロンプトはまだ Step 7 の範囲

## 想定確認手順

1. `backend/` でサーバー起動
2. ブラウザまたはクライアントで `/ws/chat?token=...` に接続
3. 低い `moisture_percent` で一度 `POST /api/v1/sensor`
4. 3分以内に `20` ポイント以上高い `moisture_percent` で再度 `POST /api/v1/sensor`
5. WebSocket に `source=sensor_event` の `reply_chunk` / `reply_done` が届くことを確認
6. Push Subscription が登録済みかつ VAPID 設定済みなら Push 通知も届くことを確認

