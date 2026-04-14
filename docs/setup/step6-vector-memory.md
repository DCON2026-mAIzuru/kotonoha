# Step 6: ベクトル検索連携 (1往復単位の保存) 構築メモ

## 今回追加したもの

- `internal/llm` の埋め込み生成レイヤー
  - `Embedder` インターフェース
  - `GeminiEmbedder`
  - `FallbackEmbedder`
- `bonsai_memories` 用のモデルとリポジトリ
  - 1往復単位の保存
  - ベクトル類似検索
- `ChatService` の永続化処理
  - `chat_history` に `user` / `assistant` を保存
  - 1往復を要約文字列にして埋め込み化し、`bonsai_memories` に保存
  - 次の Step 7 で使うための `SearchRelevantMemories` を追加

## この段階で変わったこと

通常チャットが完了すると、以下が保存されるようになりました。

- `chat_history`
  - ユーザー発話を `role=user` で保存
  - LLM の最終返答全文を `role=assistant` で保存
- `bonsai_memories`
  - `ユーザー: ... / 盆栽: ...` の1往復単位で保存
  - `source_message_ids` に元の `chat_history` 2件の ID を保持

## 保存イメージ

### chat_history

- 1件目
  - `role=user`
  - `content=こんにちは`
- 2件目
  - `role=assistant`
  - `content=こんにちは。今日も葉先までやわらかい光を感じています。`

### bonsai_memories

```text
ユーザー: こんにちは
盆栽: こんにちは。今日も葉先までやわらかい光を感じています。
```

## 埋め込みモデル

- 本番系
  - `GEMINI_API_KEY` がある場合は `GEMINI_EMBEDDING_MODEL` を使って Gemini API の `embedContent` を呼ぶ
- フォールバック
  - APIキー未設定時はローカルの `FallbackEmbedder` で決定論的なダミーベクトルを作る

## 追加した内部API

- `ChatService.SearchRelevantMemories(ctx, userID, query, limit)`
  - 検索クエリを埋め込み化して `bonsai_memories` から類似上位を返す

この検索結果はまだ通常チャットのプロンプトには混ぜていません。  
実際の RAG 応答反映は Step 7 で行います。

## 現時点の注意

- `bonsai_memories` 保存は通常チャットの完了後に実行される
- 長期記憶保存に失敗しても、チャット自体は完了させる方針
- 類似検索は Supabase の `match_bonsai_memories` SQL 関数ではなく、バックエンドからの直接 SQL で実行している
  - 現在のバックエンド接続では `auth.uid()` ベースの SQL 関数をそのまま使いにくいため
- プロフィール、現在センサー状態、検索結果をまとめてプロンプトへ入れるのは Step 7 の範囲

## 想定確認手順

1. `backend/` でサーバー起動
2. WebSocket で `/ws/chat?token=...` に接続
3. 何度か通常メッセージを送る
4. `chat_history` に `user` と `assistant` が保存されることを確認
5. `bonsai_memories` に1往復単位のレコードが増えることを確認
6. 必要なら `ChatService.SearchRelevantMemories` を Step 7 で呼び出し、関連会話が返ることを確認

