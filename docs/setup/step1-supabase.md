# Step 1: Supabase DB & Auth 構築メモ

## 目的

`docs/kotonoha-system.md` の Step 1 を進めるためのセットアップ手順です。  
この段階では、`Supabase Auth + Googleログイン` と初期DBスキーマを整えます。

## 今回追加したもの

- `supabase/migrations/20260413230000_init.sql`
  初期テーブル、RLS、pgvector拡張、更新日時トリガー、長期記憶検索関数
- `.env.example`
  フロントエンド、バックエンド、Gemini、Web Push で使う環境変数のひな形

## 前提

- Supabase プロジェクトを新規作成済みであること
- Google Cloud 側で OAuth クライアントを作成できること
- ローカルでは `frontend` を `http://localhost:3000`、`backend` を `http://localhost:8080` で動かす想定

## 作業手順

### 1. Supabase プロジェクト作成

1. Supabase で新しいプロジェクトを作成する
2. `Project URL` と `Publishable key`、`Secret key` を控える
3. それぞれ `.env.example` を参考に実際の `.env` へ設定する

### 2. Googleログインを有効化

1. Supabase ダッシュボードの `Authentication` を開く
2. `Providers` から `Google` を有効化する
3. Google Cloud で OAuth 同意画面と OAuth クライアントを作成する
4. Supabase ダッシュボードに表示されるコールバックURLを Google Cloud 側のリダイレクトURIに設定する
5. ローカル開発用のリダイレクトURLとして、少なくともフロント側のコールバックURLを追加する

## 3. SQL の適用

以下のどちらかで `supabase/migrations/20260413230000_init.sql` を適用します。

- Supabase ダッシュボードの SQL Editor に貼り付けて実行する
- Supabase CLI を使ってマイグレーションを流す

## 4. 作成されるテーブル

- `bonsai_profiles`
  盆栽名と性格設定
- `push_subscriptions`
  Push購読先
- `sensor_data`
  ユーザーごとの最新センサー状態
- `chat_history`
  短期記憶として使う会話履歴
- `bonsai_memories`
  pgvector を使った長期記憶

## 5. RLS 方針

- すべてのテーブルで `auth.uid() = user_id` のポリシーを作成済み
- フロントエンドから直接読む場合でも、自分のデータだけ参照できる
- バックエンドから `Secret key` でアクセスする場合は RLS をバイパスできる

## 6. 今の前提

- 1ユーザーにつき 1盆栽
- `sensor_data` は履歴ではなく最新状態のみ保持
- ベクトル次元はひとまず `768` を採用
  Gemini 系の埋め込みを前提にしているが、変更する場合はマイグレーション追加で調整する

## 7. Step 2 に渡す前提

次のバックエンド実装では、以下を前提に進められます。

- `Supabase JWT` を使ってプロフィールAPIと WebSocket を保護する
- センサーAPIは `X-Sensor-Token` で固定トークン認証にする
- `sensor_data` の最新値を会話生成時のコンテキストに使う

## 補足

- 最新の Supabase では、従来の `anon key` の代わりに `Publishable key` が案内されることがある
- 同様に、`service_role key` の代わりに `Secret key` が案内されることがある
- このプロジェクトでは新しい表記に合わせて `PUBLISHABLE_KEY` と `SECRET_KEY` を使う
