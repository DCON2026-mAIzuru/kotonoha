# ソフトウェア進捗

最終更新: 2026-04-28

## 現在の実装概要

Kotonoha は、Next.js フロントエンド、Go/Gin バックエンド、Supabase PostgreSQL、Gemini、WebSocket、Web Push を使って、盆栽と会話する体験を実装している。

現時点では、認証、盆栽プロフィール、センサー取り込み、WebSocket ストリーミングチャット、会話履歴、長期記憶 RAG、Push 通知、PWA、親育てチャット向けのなじみ具合制御まで入っている。

## 実装済み

### 1. 認証とプロフィール

- Supabase Auth の JWT を backend middleware で検証する
- `/api/v1/auth/me` でログインユーザー確認ができる
- `/api/v1/profile` で盆栽プロフィールを取得・保存できる
- プロフィール項目は以下を保持する
  - `name`
  - `personality`
  - `backstory`
  - `tone_mode`
  - `bond_score`
  - `bond_updated_at`
- `backstory` により、盆栽の来歴や前の持ち主との思い出を system prompt に混ぜられる

関連ファイル:

- `backend/internal/api/middleware/auth_middleware.go`
- `backend/internal/service/profile_service.go`
- `backend/internal/models/bonsai_profile.go`
- `backend/internal/repository/bonsai_profile_repository.go`
- `supabase/migrations/20260424120000_add_bonsai_profile_backstory.sql`
- `supabase/migrations/20260427000000_add_parenting_chat_state.sql`

### 2. チャットと LLM ストリーミング

- `/ws/chat?token=...` で認証付き WebSocket チャットを行う
- frontend から `type=message` を送ると、backend が Gemini の応答を `reply_chunk` で逐次返す
- 応答完了時は `reply_done` を返す
- 応答開始前に `reply_start` を返し、typing 情報を frontend に渡す
- WebSocket は ping/pong と書き込みロックで接続維持をしている
- LLM streamer は抽象化され、Gemini と fallback 実装を切り替えられる

関連ファイル:

- `backend/internal/api/handlers/chat_handler.go`
- `backend/internal/service/chat_service.go`
- `backend/internal/llm/streamer.go`
- `backend/internal/llm/gemini_streamer.go`
- `backend/internal/llm/fallback_streamer.go`
- `frontend/src/hooks/use-chat-socket.ts`

### 3. Parenting Chat Prompt

- `backend/internal/prompts/parenting_chat.md` を runtime prompt として読み込む
- system prompt は以下の構造で組み立てている
  - 基本原則
  - この個体の樹格
  - 今日の身体
  - 育ちの記録
  - 手入れの記録
  - なじみ具合
  - 根に残る会話の欠片
  - 直近の会話
  - 今日の話し方
- センサー数値は直接読ませず、身体状態として自然言語化する
- `bond_score >= 90` かつ条件を満たす時だけ、「ぼんす」語尾を一文まで許可する
- `tone_mode` は `gentle`, `childlike`, `calm`, `classical` を backend で正規化して prompt に反映する

関連ファイル:

- `backend/internal/prompts/parenting_chat.md`
- `backend/internal/service/chat_prompt.go`
- `backend/internal/service/chat_service.go`
- `backend/internal/service/bond_service.go`

### 4. なじみ具合

- `bonsai_profiles` に `bond_score` を保存する
- `bond_score` は 0 から 100 に丸める
- `bond_score` から内部 `bond_level` を計算する
  - `cautious`
  - `aware`
  - `familiar`
  - `attached`
  - `trusted`
- チャット完了時に `talked` として `bond_score +1`
- 水やりイベントで `bond_score +2`
- 低水分イベントで `bond_score -1`
- `reply_start.typing` で、なじみ具合に応じた表示待ち時間と表示速度を返す
- センサー状態が不調の場合は返答開始を少し遅くし、速すぎる表示を抑える

関連ファイル:

- `backend/internal/service/bond_service.go`
- `backend/internal/service/chat_service.go`
- `backend/internal/service/sensor_service.go`
- `frontend/src/types/api.ts`
- `frontend/src/hooks/use-chat-socket.ts`

### 5. 会話履歴と長期記憶

- `chat_history` に user / assistant / system / event の履歴を保存する
- `/api/v1/chat/history` で直近履歴を取得できる
- `/api/v1/chat/history` の DELETE で会話履歴と長期記憶を削除できる
- 1往復の会話を要約し、Gemini embedding で `bonsai_memories` に保存する
- 次の会話時に関連記憶を検索して prompt の「根に残る会話の欠片」に入れる
- pgvector は 3072 次元に移行済み

関連ファイル:

- `backend/internal/service/chat_service.go`
- `backend/internal/repository/chat_history_repository.go`
- `backend/internal/repository/bonsai_memory_repository.go`
- `backend/internal/llm/embedder.go`
- `backend/internal/llm/gemini_embedder.go`
- `supabase/migrations/20260415231000_migrate_bonsai_embedding_to_3072.sql`

### 6. センサー取り込みとイベント

- `/api/v1/sensor` に ESP32 または mock からセンサー値を POST できる
- 必須ヘッダーは `X-Sensor-Token` と `X-Sensor-User-ID`
- センサー値は user ごとに最新値を保持する
- `recorded_at` が古い値は保存しない
- 異常値や古い値を `sensor_status` として扱う
- 短時間の水分上昇で `watering_detected` を検知する
- 低水分で `low_moisture_detected` を検知する
- イベントは 2 時間のクールダウンを持つ
- イベント発生時は LLM で短い通知文を生成し、WebSocket と Push 通知へ流す
- イベント履歴は `chat_history.role = event` として保存する

関連ファイル:

- `backend/internal/api/handlers/sensor_handler.go`
- `backend/internal/service/sensor_service.go`
- `backend/internal/repository/sensor_repository.go`
- `hardware/esp32_sensor/esp32_sensor.ino`
- `hardware/simulator/sensor_simulator.py`
- `esp/esp_mock_sensor/esp_mock_sensor.ino`

### 7. センサー手動固定

- frontend から水分値を一時的に固定できる
- `/api/v1/sensor/moisture-control` で固定状態の取得・更新ができる
- 固定中は backend 側で水分値を manual value に差し替える
- raw payload に固定中であることと手動水分値を残す

関連ファイル:

- `backend/internal/service/sensor_service.go`
- `backend/internal/api/handlers/sensor_handler.go`
- `frontend/src/components/chat-panel.tsx`
- `frontend/src/lib/backend-api.ts`

### 8. Web Push と PWA

- Push 購読情報を保存・削除できる
- デモ Push 送信 API がある
- センサーイベント時に登録済み購読へ Push 通知を送る
- PWA manifest と service worker を配置済み
- frontend で Push 許可カードと PWA インストール導線を表示する

関連ファイル:

- `backend/internal/service/push_subscription_service.go`
- `backend/internal/repository/push_subscription_repository.go`
- `backend/internal/webpush/sender.go`
- `frontend/src/components/push-permission-card.tsx`
- `frontend/src/components/pwa-install-button.tsx`
- `frontend/public/sw.js`
- `frontend/public/manifest.webmanifest`

### 9. フロントエンド UI

- Supabase ログイン後にプロフィール設定、チャット、センサー状態、Push 設定を使える
- チャット履歴を初回ロードする
- `reply_start` の `delay_ms` と `speed` を受け、文字をタイプライター表示する
- センサー状態は 30 秒ごとに更新する
- 水分固定 UI、クイックプロンプト、モバイル用の情報/センサーパネルを持つ
- プロフィール編集画面でなじみ具合を日本語ラベルとバーで表示する
- 会話履歴と長期記憶の削除ボタンを持つ

関連ファイル:

- `frontend/src/components/kotonoha-app.tsx`
- `frontend/src/components/chat-panel.tsx`
- `frontend/src/components/profile-setup-form.tsx`
- `frontend/src/hooks/use-auth.ts`
- `frontend/src/hooks/use-chat-socket.ts`
- `frontend/src/app/globals.css`

## 一部実装済み

### tone_mode

- DB、model、repository、service、API 型、prompt 反映は実装済み
- frontend のプロフィール保存では `tone_mode` を送っていないため、現状は backend の default/normalize により `gentle` になる
- UI から `childlike`, `calm`, `classical` を選ぶ画面は未実装

### growth_state / care_history

- 専用テーブルは未実装
- 現状は `bond_score`、センサー状態、`chat_history.role = event` を使って prompt 用の簡易ブロックを生成している

### なじみ具合の制御

- score 更新と prompt 反映、typing 反映は実装済み
- 日次上限や DB 側での増減制御は未実装
- 現状はアプリ側で単純に差分を加算・減算している

## 未実装 / 次に見るべきところ

- `tone_mode` を UI から選択できるようにする
- `bond_score` の日次上限、連打対策、DB 側の安全な差分更新を入れる
- `care_events` 専用テーブルを作るか、引き続き `chat_history.role = event` を使うか決める
- `growth_state` を永続化するか、prompt 内の簡易生成に留めるか決める
- センサー手動固定は現在メモリ + sensor_data 更新のため、永続化方針を決める
- 低水分イベントと水やりイベントの閾値を実機値で調整する
- ESP32 実機センサー値への置き換えと最終接続確認を行う
- frontend のなじみ具合表示は数値も出ているため、体験として数値を隠すか検討する
- TTS は設計メモ段階で、実装はまだ入っていない

## 検証状況

- backend には service 層中心の Go テストがある
  - `chat_service_test.go`
  - `sensor_service_test.go`
  - `push_subscription_service_test.go`
  - `streamer_test.go`
- frontend は TypeScript 型で `reply_start`, `typing`, `bond_score`, `tone_mode` に追従済み
- この進捗メモ更新では、ドキュメントのみ変更したためテストは実行していない
