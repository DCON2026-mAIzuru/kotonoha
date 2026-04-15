# Step 9: フロントエンド UI & PWA (Vercel) 構築メモ

## 今回追加したもの

- Next.js (App Router) + TypeScript + Tailwind のフロントエンド基盤
  - `frontend/package.json`
  - `frontend/next.config.mjs`
  - `frontend/tsconfig.json`
  - `frontend/tailwind.config.ts`
  - `frontend/src/app/layout.tsx`
  - `frontend/src/app/page.tsx`
  - `frontend/src/app/globals.css`
- Supabase Auth ログイン導線
  - メール/パスワードのログイン・新規登録
  - Google ログイン
  - `frontend/src/hooks/use-auth.ts`
  - `frontend/src/components/auth-panel.tsx`
- 初期設定画面（プロフィール登録）
  - `GET /api/v1/profile` で既存プロフィール確認
  - 未設定時に名前・性格を入力して `PUT /api/v1/profile`
  - `frontend/src/components/profile-setup-form.tsx`
- PWA / Web Push 対応
  - `frontend/public/manifest.webmanifest`
  - `frontend/public/sw.js`
  - `frontend/src/components/pwa-registrar.tsx`
  - `frontend/src/components/push-permission-card.tsx`
- WebSocket ストリーミングUI
  - `frontend/src/hooks/use-chat-socket.ts`
  - `frontend/src/components/chat-panel.tsx`
  - `reply_chunk` を順次連結してチャット欄へ表示

## この段階で変わったこと

- フロントエンド単体で、以下の一連の体験が成立しました。
  1. ログイン
  2. 初期プロフィール設定
  3. Push通知の許可と購読登録
  4. WebSocketでストリーミングチャット
- バックエンドの既存API (`/api/v1/profile`, `/api/v1/push/subscribe`, `/ws/chat`) と接続できる構成になりました。

## 必要な環境変数

`frontend/.env.example` を参考に、ローカルまたはVercelに以下を設定します。

- `NEXT_PUBLIC_SUPABASE_URL`
- `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY`
- `NEXT_PUBLIC_API_BASE_URL`
- `NEXT_PUBLIC_WS_URL`
- `NEXT_PUBLIC_VAPID_PUBLIC_KEY`

例（ローカル）:

```bash
NEXT_PUBLIC_SUPABASE_URL=https://your-project.supabase.co
NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY=your-supabase-publishable-key
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
NEXT_PUBLIC_WS_URL=ws://localhost:8080/ws/chat
NEXT_PUBLIC_VAPID_PUBLIC_KEY=your-vapid-public-key
```

## ローカル起動手順

```bash
cd frontend
npm install
npm run dev
```

ブラウザで `http://localhost:3000` を開いて確認します。

## 想定確認手順

1. ログイン画面で Supabase Auth にログインする
2. 初回はプロフィール設定フォームが表示されることを確認する
3. 設定保存後にチャット画面へ遷移することを確認する
4. Push通知カードで許可を取り、`/api/v1/push/subscribe` に保存されることを確認する
5. チャット送信後、`reply_chunk` が順次表示され、最後に応答が完成することを確認する

## Vercel デプロイ手順（概要）

1. Vercel に `frontend` ディレクトリをプロジェクトとして接続
2. Build Command: `npm run build`
3. Output: Next.js 既定値
4. 環境変数に `.env.example` の `NEXT_PUBLIC_*` を設定
5. デプロイ後、バックエンド側の `BACKEND_ALLOWED_ORIGIN` にフロントの本番ドメインを設定

## 現時点の注意

- Push購読には `NEXT_PUBLIC_VAPID_PUBLIC_KEY` が必要
- iOS/Safari のPushはOS・ブラウザ条件に依存するため、Android Chromeでの確認を優先すると安定
- WebSocket接続先は本番時に `wss://.../ws/chat` を指定する
- CORS制限により、バックエンドの `BACKEND_ALLOWED_ORIGIN` とフロントドメインを一致させる必要がある
