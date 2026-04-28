# Kotonoha

盆栽と会話する体験を作るプロジェクトです。

ESP32 センサー値をコンテキストとして取り込み、LLM で「盆栽らしい返答」をストリーミング表示します。さらに会話履歴を長期記憶としてベクトル化し、関連記憶を使った RAG 応答と Push 通知連携を行います。

## 現在の実装状況

- Step 1 から Step 9 まで実装済み
- Step 10（ESP32 実機側コードの最終整備・接続確認）は今後の作業

## 主な機能

- Supabase Auth（メール/パスワード、Google ログイン）
- 盆栽プロフィール設定（名前・性格）
- WebSocket ストリーミングチャット
- センサー API 取り込みと水やり・乾燥イベント検知
- Web Push 通知（VAPID）
- 会話ログ保存（短期）
- 1往復単位のベクトル記憶（長期）
- RAG + 動的プロンプト（プロフィール、センサー状態、履歴、関連記憶を反映）

## 技術スタック

- Frontend: Next.js (App Router), TypeScript, Tailwind CSS, Supabase JS
- Backend: Go, Gin, Gorilla WebSocket, pgx
- Database: Supabase PostgreSQL + pgvector
- AI: Gemini（ストリーミング / 埋め込み）
- Push: webpush-go
- Deploy: Frontend は Vercel、Backend は Fly.io

## リポジトリ構成

```text
.
├── backend/     # Go API, WebSocket, LLM, Service層
├── frontend/    # Next.js UI, PWA, Push登録, Supabase Auth
├── supabase/    # SQL migration
├── hardware/    # ESP32 関連
└── docs/        # 設計・実装メモ
```

## 事前準備

- Node.js 18 以上（推奨: 20 以上）
- npm
- Go（backend/go.mod に準拠）
- Supabase プロジェクト

## セットアップ

### 1. Supabase 初期化

1. Supabase プロジェクトを作成
2. Auth の Google Provider を有効化（必要なら）
3. 次の SQL を適用

- supabase/migrations/20260413230000_init.sql

### 2. 環境変数

ひな形:

- .env.example
- frontend/.env.example

推奨配置:

- backend/.env
- frontend/.env.local

最低限必要な値（抜粋）:

- SUPABASE_URL
- SUPABASE_PUBLISHABLE_KEY
- SUPABASE_SECRET_KEY
- SUPABASE_DB_URL
- NEXT_PUBLIC_SUPABASE_URL
- NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY
- NEXT_PUBLIC_API_BASE_URL
- NEXT_PUBLIC_WS_URL

必要に応じて:

- SUPABASE_JWT_SECRET
- SENSOR_SHARED_TOKEN
- GEMINI_API_KEY
- GEMINI_MODEL
- GEMINI_MODEL_PRIORITY
- GEMINI_EMBEDDING_MODEL
- VAPID_PUBLIC_KEY
- VAPID_PRIVATE_KEY
- VAPID_SUBJECT
- NEXT_PUBLIC_VAPID_PUBLIC_KEY

## ローカル起動

### Backend

```bash
cd backend
go mod tidy
go run ./cmd/server
```

ホットリロードを使う場合:

```bash
cd backend
go install github.com/air-verse/air@latest
air
```

`backend/.air.toml` が `cmd/server` を監視・再ビルドします。`internal/prompts/*.md` も監視対象なので、実行用プロンプトを編集した場合も自動で再起動されます。

起動後の確認:

- GET /healthz
- GET /readyz

### Frontend

```bash
cd frontend
npm install
npm run dev
```

- http://localhost:3000 を開く

## 主要 API / WebSocket

### Health

- GET /healthz
- GET /readyz

### Auth

- GET /api/v1/auth/me

### Profile

- GET /api/v1/profile
- PUT /api/v1/profile

### Push Subscription

- GET /api/v1/push/subscriptions
- POST /api/v1/push/subscribe
- POST /api/v1/push/demo
- DELETE /api/v1/push/subscriptions/:id

### Sensor

- POST /api/v1/sensor
- 必須ヘッダー:
  - X-Sensor-Token
  - X-Sensor-User-ID

例:

```json
{
  "device_id": "mock-device-001",
  "recorded_at": "2026-04-14T10:30:00+09:00",
  "moisture_percent": 62.5,
  "light_lux": 9800,
  "temperature_celsius": 24.1
}
```

### WebSocket Chat

- GET /ws/chat?token=<Supabase JWT>

クライアント送信:

```json
{ "type": "message", "text": "おはよう" }
```

サーバー受信（ストリーミング中）:

```json
{ "type": "reply_chunk", "text": "おは" }
```

サーバー受信（完了）:

```json
{ "type": "reply_done" }
```

## テスト / 検証

### Backend

```bash
cd backend
go test ./...
```

### Frontend

```bash
cd frontend
npm run lint
npm run build
```

## デプロイ

### Backend (Fly.io)

- backend/fly.toml を利用
- backend/Dockerfile でコンテナビルド

例:

```bash
cd backend
fly apps create <your-app-name>
fly secrets set KEY="VALUE"
fly deploy
```

### Frontend (Vercel)

- frontend ディレクトリをプロジェクトとして接続
- NEXT*PUBLIC*\* の環境変数を設定
- backend の BACKEND_ALLOWED_ORIGIN をフロント本番ドメインに合わせる

## ドキュメント

- docs/kotonoha-system.md
- docs/setup/step1-supabase.md
- docs/setup/step2-backend.md
- docs/setup/step3-profile-and-push.md
- docs/setup/step4-websocket-and-llm-streaming.md
- docs/setup/step5-sensor-event-and-push.md
- docs/setup/step6-vector-memory.md
- docs/setup/step7-rag-chat-and-dynamic-prompt.md
- docs/setup/step8-backend-flyio.md
- docs/setup/step9-frontend-ui-and-pwa.md

## 注意事項

- 機密情報はリポジトリにコミットしないでください
- もし API キーや Secret が露出した場合は再発行してください
- Next.js 15 系では next lint の将来非推奨告知が出るため、将来的に ESLint CLI への移行を検討してください
