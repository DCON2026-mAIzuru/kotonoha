# Step 4: WebSocket & LLMストリーミング基盤 構築メモ

## 今回追加したもの

- `internal/llm` パッケージ
  - `Streamer` インターフェース
  - `GeminiStreamer`（Gemini Streaming API 呼び出し）
  - `FallbackStreamer`（APIキー未設定時の動作確認用）
- `ChatService`
  - WebSocket から受けたメッセージを LLM ストリーミングに中継
- `ChatHandler`
  - `GET /ws/chat` で WebSocket 接続を受け付け
  - `reply_chunk` / `reply_done` を順次送信
  - ping/pong による接続維持
- ルーターと DI の更新
  - `/ws/chat` を JWT 認証付きで有効化

## 追加されたエンドポイント

- `GET /ws/chat?token=<Supabase JWT>`
  WebSocket でチャット接続する

## メッセージフォーマット

### クライアント -> サーバー

```json
{
  "type": "message",
  "text": "おはよう"
}
```

### サーバー -> クライアント（ストリーミング中）

```json
{
  "type": "reply_chunk",
  "text": "おは"
}
```

### サーバー -> クライアント（完了）

```json
{
  "type": "reply_done"
}
```

### サーバー -> クライアント（エラー）

```json
{
  "type": "error",
  "message": "エラーメッセージ"
}
```

## 現時点の注意

- `GEMINI_API_KEY` が未設定の場合は `FallbackStreamer` が動作する
- この段階では会話履歴の保存や RAG 連携はまだ未実装（Step 6/7で実施）
- プロファイル（名前/性格）のプロンプト反映はまだ未実装（Step 7で実施）

## 想定確認手順

1. `backend/` でサーバー起動
2. WebSocket で `/ws/chat?token=...` に接続
3. `{"type":"message","text":"こんにちは"}` を送信
4. `reply_chunk` が複数回届き、最後に `reply_done` が届くことを確認
