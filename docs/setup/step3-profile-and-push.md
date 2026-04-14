# Step 3: Web Push & 初期設定 API 構築メモ

## 今回追加したもの

- `bonsai_profiles` を扱う取得・更新 API
- `push_subscriptions` を扱う一覧・登録・削除 API
- `ProfileService`, `PushSubscriptionService`
- `PostgresStore` 用のプロフィール / Push 購読リポジトリ

## 追加されたエンドポイント

- `GET /api/v1/profile`
  ログイン中ユーザーの盆栽プロフィールを取得する
- `PUT /api/v1/profile`
  盆栽名と性格設定を保存・更新する
- `GET /api/v1/push/subscriptions`
  登録済み Push 購読一覧を取得する
- `POST /api/v1/push/subscribe`
  Push Subscription を保存・更新する
- `DELETE /api/v1/push/subscriptions/:id`
  Push Subscription を削除する

## リクエスト例

### プロフィール保存

```json
{
  "name": "言ノ葉",
  "personality": "静かで古風だけれど、親しい相手にはやわらかく話す"
}
```

### Push購読保存

```json
{
  "endpoint": "https://fcm.googleapis.com/fcm/send/xxxxx",
  "subscription": {
    "endpoint": "https://fcm.googleapis.com/fcm/send/xxxxx",
    "expirationTime": null,
    "keys": {
      "p256dh": "...",
      "auth": "..."
    }
  }
}
```

## 現時点の注意

- この段階では Push の「送信」はまだ未実装
- まずはフロントエンドが購読情報を保存できる状態を作っている
- プロフィールは `PUT` で upsert するので、初回作成と更新を同じAPIで扱える

## Step 4 に渡す前提

- 初回ログイン後のプロフィール設定保存先ができた
- Push Subscription の保存先ができた
- 次は WebSocket と LLM ストリーミング基盤に進める
