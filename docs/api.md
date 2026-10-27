# API と WebSocket

この一覧の根拠は `backend/internal/api/router.go` です。レスポンスの詳細や検証条件は各ハンドラーとモデルを参照してください。

## Swagger UI

Swagger の実装が含まれるブランチでは、backend 起動後に [http://localhost:8080/swagger/](http://localhost:8080/swagger/) で Swagger UI を開けます。OpenAPI 仕様は `/swagger/openapi.json` から取得できます。仕様は `backend/internal/api/apidocs/openapi.json` で管理しているため、API の変更時はあわせて更新してください。

認証が必要な API は Swagger UI の **Authorize** に Supabase のアクセストークンを入力します。センサー送信には `X-Sensor-Token` と `X-Sensor-User-ID` が必要です。Swagger UI では WebSocket の送受信は実行できません。UI の表示には CDN への接続が必要です。ドキュメント自体は認証なしで公開されますが、各 API の認証は従来どおりです。

現在のブランチには Swagger のルートと仕様ファイルがまだ含まれていません。利用する際は Swagger 実装を含むブランチを確認してください。

## 認証とエンドポイント

`/healthz` と `/readyz` は認証不要です。`POST /api/v1/sensor` はセンサー専用の `X-Sensor-Token` と `X-Sensor-User-ID` を使います。それ以外の REST API は `Authorization: Bearer <Supabase access token>` を付けます。WebSocket はクエリパラメーター `token` でも認証できます。

| 分野 | エンドポイント |
| --- | --- |
| 稼働確認 | `GET /healthz`、`GET /readyz`（DB 接続確認） |
| 認証・プロフィール | `GET /api/v1/auth/me`、`GET /api/v1/profile`、`PUT /api/v1/profile` |
| チャット | `GET /api/v1/chat/history`、`DELETE /api/v1/chat/history`、`GET /ws/chat`（WebSocket） |
| センサー | `GET /api/v1/sensor`、`POST /api/v1/sensor`、`GET` / `PATCH /api/v1/sensor/moisture-control`、`GET` / `POST /api/v1/sensor/presentation-demo` |
| 画像診断 | `POST /api/v1/diagnosis/images`、`GET /api/v1/diagnosis/latest`、`GET /api/v1/diagnosis/history`、`GET /api/v1/diagnosis/artifacts/:id/:kind` |
| 音声合成 | `POST /api/v1/tts/voicevox` |
| Push | `GET /api/v1/push/subscriptions`、`POST /api/v1/push/subscribe`、`POST /api/v1/push/demo`、`DELETE /api/v1/push/subscriptions/:id` |

発表用センサーデモの操作にはバックエンドの `PRESENTATION_DEMO_MODE=true` が必要です。センサーの送信形式は [センサー送信](setup/sensors.md) を参照してください。

## チャット

`/ws/chat?token=<Supabase access token>` に接続し、次の JSON を送信します。

```json
{"type":"message","text":"おはよう"}
```

応答は `reply_start`、テキストを含む `reply_chunk`、`reply_done` の順に届きます。センサーイベントなどのメッセージやエラーも同じ接続で受信します。詳細な形式は `backend/internal/realtime/hub.go` と各ハンドラーを参照してください。
