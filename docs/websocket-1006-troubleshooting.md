# WebSocket code=1006 切断時の調査手順

対象: frontend で以下が表示される場合

- WebSocket接続が切断されました (code=1006, endpoint=ws://localhost:8080/ws/chat)

この `1006` は、クライアントが正常な close frame を受け取れず接続断になったときのブラウザ側コードです。
このプロジェクトでは、実際には「ハンドシェイク前に 401/403 が返っている」ケースが多いです。

## まず結論

次の2点を満たせば接続できます。

1. backend が 8080 で正常起動している
2. 有効な Supabase access token を付けて `/ws/chat` に接続している

今回実際に確認した事象:

- Supabase `/auth/v1/user` が `403 session_not_found` を返すと、backend 側は `認証トークンを検証できませんでした` になり、frontend では `code=1006` に見える
- つまり token の署名方式の問題ではなく、セッションが既に無効化されているのが原因

## 1. backend 起動状態を確認

```bash
cd backend
go run ./cmd/server
```

別ターミナルで:

```bash
curl -i http://localhost:8080/healthz
```

期待値:

- `200 OK`

もし `bind: address already in use` が出る場合:

```bash
lsof -nP -iTCP:8080 -sTCP:LISTEN
```

8080 を占有している古いプロセスを止めてから再起動する。

## 2. WebSocket 受け口が生きているか確認

```bash
curl -i http://localhost:8080/ws/chat
```

期待値:

- `401 Unauthorized`
- `{"message":"認証トークンがありません"}`

これは正常。受け口自体は生きている。

## 3. token が backend で有効か確認

frontend でログイン後に access token を使って確認:

```bash
curl -i http://localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer <ACCESS_TOKEN>"
```

期待値:

- `200 OK`
- user 情報 JSON

`401` の場合は token が無効/期限切れ/別環境のもの。

`403 session_not_found`（Supabase直叩き時）の場合は、トークンの `session_id` が既に失効している。
この場合は再ログインが必須。

## 4. token 付き WebSocket を直接確認

```bash
node - <<'NODE'
const token = '<ACCESS_TOKEN>';
const ws = new WebSocket(`ws://localhost:8080/ws/chat?token=${encodeURIComponent(token)}`, {
  headers: { Origin: 'http://localhost:3000' },
});
ws.onopen = () => {
  console.log('OPEN');
  ws.close();
};
ws.onerror = (e) => console.log('ERROR', e?.message || e);
ws.onclose = (e) => console.log('CLOSE', e.code, e.reason || '(no reason)');
NODE
```

期待値:

- `OPEN`

ここで `OPEN` するのにブラウザだけ `1006` の場合は、frontend 側の状態不整合が原因。

## 5. frontend 側の復旧手順

1. アプリでログアウト
2. `npm run dev` を再起動
3. ブラウザをハードリロード
4. 再ログイン
5. 画面の接続先表示が `ws://localhost:8080/ws/chat` か確認

## 6. backend ログで最終判定

`go run ./cmd/server` のログに注目:

- `GET /ws/chat?... | 200` ならハンドシェイク成功
- `GET /ws/chat?... | 401` なら認証失敗

## よくある原因

- backend が実は起動していない
- 8080 を古いプロセスが占有している
- token が期限切れ
- Supabase 側で session が失効している（`session_not_found`）
- ログイン直後に古い token を参照している
- frontend が古いビルド/キャッシュを参照している

## 共有してもらうと切り分けが早い情報

不具合報告時は以下3点をセットで共有:

1. `GET /ws/chat?...` の backend ログ1行
2. `GET /api/v1/auth/me` の結果
3. 画面の endpoint 表示
