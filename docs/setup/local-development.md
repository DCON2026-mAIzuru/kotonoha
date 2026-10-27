# ローカル開発・運用

この手順は現行のコードと環境変数に基づきます。コマンドは特記がなければリポジトリのルートから実行してください。

## 事前準備

- Node.js と npm（Next.js 15 の要件を満たす Node.js 18.18 以上。Node.js 20 以上を推奨）
- Go（`backend/go.mod` に指定されたバージョン）
- Supabase プロジェクトと PostgreSQL への接続情報
- 画像診断を使う場合は Python、読み上げを使う場合は VOICEVOX Engine

## セットアップ

### 1. Supabase

Supabase プロジェクトを作成し、`supabase/migrations/` 内の SQL を**ファイル名の昇順ですべて**適用します。Supabase SQL Editor または Supabase CLI を利用してください。Google ログインを使う場合は、Google Cloud で OAuth 同意画面と OAuth クライアントを作成し、Supabase Auth の Google Provider に設定します。Supabase が表示するコールバック URL を Google Cloud のリダイレクト URI に登録し、Supabase の URL 設定にフロントエンドのリダイレクト先を追加してください。

### 2. 環境変数

リポジトリのルートでひな形をコピーし、それぞれのファイルを編集します。

```bash
cp .env.example backend/.env
cp frontend/.env.example frontend/.env.local
```

`backend/.env` には最低限 `SUPABASE_URL`、`SUPABASE_PUBLISHABLE_KEY`、`SUPABASE_SECRET_KEY`、`SUPABASE_DB_URL` を設定してください。`frontend/.env.local` には `NEXT_PUBLIC_SUPABASE_URL`、`NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY`、backend の接続先を設定します。`BACKEND_ALLOWED_ORIGIN` はフロントエンドのオリジンに合わせてください。

機能に応じた主な設定は次のとおりです。

| 機能 | 設定 |
| --- | --- |
| Gemini | `GEMINI_API_KEY`、必要に応じて `GEMINI_MODEL`、`GEMINI_MODEL_PRIORITY`、`GEMINI_EMBEDDING_MODEL`。API キーが空の場合は代替の応答・埋め込みを使用します。 |
| センサー送信 | backend に `SENSOR_SHARED_TOKEN` を設定し、送信側にも同じ値を設定します。 |
| Web Push | backend に `VAPID_PUBLIC_KEY`、`VAPID_PRIVATE_KEY`、`VAPID_SUBJECT`、frontend に `NEXT_PUBLIC_VAPID_PUBLIC_KEY` を設定します。公開鍵は同じものを使います。 |
| 画像診断 | `DL_SERVICE_URL`（既定 `http://127.0.0.1:8090`）、`DL_SERVICE_TIMEOUT_SECONDS`、`DIAGNOSIS_MAX_IMAGE_MB`、`DIAGNOSIS_ARTIFACT_DIR` を必要に応じて設定します。 |
| 音声合成 | `VOICEVOX_ENGINE_URL`（既定 `http://127.0.0.1:50021`）、`VOICEVOX_SPEAKER_ID`、`VOICEVOX_SPEAKER_NAME`、`VOICEVOX_STYLE_NAME`、`VOICEVOX_SPEED_SCALE`、`VOICEVOX_TIMEOUT_SECONDS` を必要に応じて設定します。 |
| 発表用操作 | backend の `PRESENTATION_DEMO_MODE` と frontend の `NEXT_PUBLIC_PRESENTATION_DEMO_MODE` を必要に応じて `true` にします。 |

`SUPABASE_JWT_SECRET` は JWT をローカル検証する場合に設定します。秘密鍵や `.env` ファイルはコミットしないでください。

## ローカル起動

各サービスは別々のターミナルで起動します。

### Backend

```bash
cd backend
go run ./cmd/server
```

既定のポートは `8080` です。`/healthz` で稼働状態、`/readyz` でデータベースへの接続を確認できます。ホットリロードを使う場合は Air をインストールし、`backend/` で `air` を実行してください。設定は `backend/.air.toml` にあります。

### Frontend

```bash
cd frontend
npm install
npm run dev
```

[http://localhost:3000](http://localhost:3000) を開きます。

### 画像診断サービス（画像診断を使う場合）

```bash
cd DL
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
uvicorn api:app --host 127.0.0.1 --port 8090
```

`http://127.0.0.1:8090/healthz` で稼働を確認できます。詳細は [画像診断サービス](image-diagnosis.md) を参照してください。

### VOICEVOX Engine（読み上げを使う場合）

VOICEVOX Engine を起動し、backend の `VOICEVOX_ENGINE_URL` をその接続先に合わせます。backend から接続できる状態にしてから、画面の読み上げ機能を利用してください。

## テスト / 検証

```bash
cd backend
go test ./...
```

```bash
cd frontend
npm run lint
npm run build
```

## デプロイ

- Backend は `backend/Dockerfile` と `backend/fly.toml` を使って Fly.io へデプロイできます。必要な環境変数は secrets に設定してください。
- Frontend は `frontend/` を Vercel のプロジェクトとして接続し、必要な `NEXT_PUBLIC_*` 変数を設定します。backend の `BACKEND_ALLOWED_ORIGIN` も本番フロントエンドに合わせてください。
- 画像診断や音声合成を使う場合は、backend から各サービスへ到達できる URL を設定してください。

