# Step 2: バックエンド基盤 (Go/Gin) 構築メモ

## 今回追加したもの

- `backend/go.mod`
  Go モジュール定義
- `backend/cmd/server/main.go`
  エントリーポイント
- `backend/internal/api/...`
  ルーター、ヘルスチェック、認証ユーザー確認用ハンドラー、JWTミドルウェア
- `backend/internal/repository/postgres.go`
  Supabase PostgreSQL 接続
- `backend/internal/service/health_service.go`
  DB疎通確認サービス
- `backend/pkg/config/config.go`
  `.env` 読み込みと必須値チェック
- `backend/pkg/auth/jwt.go`
  Supabase JWT のローカル検証

## 現時点のエンドポイント

- `GET /healthz`
  プロセスの生存確認
- `GET /readyz`
  PostgreSQL への疎通確認
- `GET /api/v1/auth/me`
  認証済みユーザーの確認

## 認証方針

- フロントからの API 呼び出しは `Authorization: Bearer <Supabase JWT>` を前提にする
- 将来の `WebSocket /ws/chat?token=...` に備えて、ミドルウェア側で `token` クエリも受け取れるようにしてある
- JWT 検証は `SUPABASE_JWT_SECRET` があればローカル検証し、未設定なら Supabase Auth の `/auth/v1/user` へ問い合わせて検証する
- センサー API はこの段階では未実装

## 追加で必要になった環境変数

- `SUPABASE_DB_URL`
  Supabase PostgreSQL へ直接接続するための接続文字列
- `SUPABASE_JWT_SECRET`
  任意。設定するとバックエンドで Supabase JWT をローカル検証できる

## 起動前に必要なこと

1. `.env` に Step 1 の値と `SUPABASE_DB_URL` を設定する
2. 可能なら `SUPABASE_JWT_SECRET` も設定する
3. `backend/` ディレクトリで依存関係を取得する
4. サーバーを起動する

## 想定コマンド

```bash
cd backend
go mod tidy
go run ./cmd/server
```

## Step 3 に渡す前提

- Gin ルーターと DI の土台はできている
- PostgreSQL へ接続できる
- JWT 認証済みユーザーをハンドラーで受け取れる
- 次は `bonsai_profiles` の CRUD と Push Subscription 登録 API を追加できる

## 補足

- 最新の Supabase では非対称の署名鍵が使われることがあるため、`JWT secret` を必須前提にはしていない
- `SUPABASE_JWT_SECRET` を使ったローカル検証は高速だが、構成によっては利用できない
- その場合でも Supabase Auth サーバー経由でトークンの妥当性確認はできる
