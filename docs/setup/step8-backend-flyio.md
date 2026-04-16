# Step 8: バックエンド Fly.io デプロイ 構築メモ

## 今回追加したもの

- `backend/Dockerfile`
  - Go バイナリをマルチステージビルドし、最終イメージを軽量化
  - `CGO_ENABLED=0` で静的リンク実行ファイルを生成
  - distroless ランタイムで非root実行
- `backend/fly.toml`
  - Fly.io のアプリ定義
  - `http_service` で `internal_port=8080` を公開
  - `/healthz` のヘルスチェックを設定
  - `min_machines_running=1` で WebSocket 常時接続向けの最低1台稼働を確保
- `backend/.dockerignore`
  - `.env` や不要ファイルをイメージに含めない設定

## この段階で変わったこと

- バックエンドをローカル開発だけでなく Fly.io 上で常時稼働できる前提が整いました。
- WebSocket を含む API (`/ws/chat`, `/api/v1/*`) を HTTPS 経由で公開できる構成になりました。
- デプロイ先でも `BACKEND_PORT=8080` で起動し、Fly のプロキシと整合するようになりました。

## 事前準備

1. Fly CLI をインストールし、ログインする

```bash
brew install flyctl
fly auth login
```

2. `backend/fly.toml` の `app` 名を一意な名前に変更する
   - 例: `kotonoha-backend-<team-or-user>`

## 初回デプロイ手順

`backend/` で実行:

```bash
cd backend
fly apps create <your-app-name>
```

`backend/fly.toml` の `app` が作成名と一致していることを確認後、必要なシークレットを登録:

```bash
fly secrets set \
  SUPABASE_URL="..." \
  SUPABASE_PUBLISHABLE_KEY="..." \
  SUPABASE_SECRET_KEY="..." \
  SUPABASE_DB_URL="..." \
  SUPABASE_JWT_SECRET="..." \
  SENSOR_SHARED_TOKEN="..." \
  GEMINI_API_KEY="..." \
  GEMINI_MODEL="gemini-2.5-flash" \
  GEMINI_EMBEDDING_MODEL="gemini-embedding-001" \
  VAPID_PUBLIC_KEY="..." \
  VAPID_PRIVATE_KEY="..." \
  VAPID_SUBJECT="mailto:you@example.com" \
  BACKEND_ALLOWED_ORIGIN="https://<frontend-domain>"
```

デプロイ実行:

```bash
fly deploy
```

## 動作確認

```bash
fly status
fly logs
curl -i https://<your-app-name>.fly.dev/healthz
curl -i https://<your-app-name>.fly.dev/readyz
```

期待値:

- `/healthz` が `200`
- `/readyz` が DB 疎通可能なとき `200`
- `fly logs` に起動ログ `バックエンドを起動しました: port=8080`

## 運用時の注意

- `BACKEND_ALLOWED_ORIGIN` はフロントエンドの本番ドメインを設定する
- WebSocket 先は `wss://<your-app-name>.fly.dev/ws/chat?token=...` を使用する
- DB 接続失敗時は `/readyz` が落ちるため、`SUPABASE_DB_URL` の値を優先して確認する
- Push 通知を有効化する場合、VAPID キーは必ず `fly secrets` 側に設定する

## 更新デプロイ

コード変更後は `backend/` で以下を実行:

```bash
fly deploy
```

シークレット変更時:

```bash
fly secrets set KEY="VALUE"
```

ロールアウト後の状態確認:

```bash
fly releases
fly machine list
```
