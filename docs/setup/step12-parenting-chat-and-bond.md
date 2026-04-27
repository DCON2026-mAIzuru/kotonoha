# Step 12: Parenting Chat と なじみ具合の実装

## このStepで実装したこと

`docs/mypronpt/BONSAI-parenting-chat-prompt.md` と `docs/mypronpt/BONSAI-parenting-chat-implementation-plan.md` を元に、既存のチャット体験へ「雑談」「世話」「育っている感覚」「なじみ具合」を追加した。

主な変更点：

- `bonsai_profiles` に `tone_mode`, `bond_score`, `bond_updated_at` を追加
- `bond_score` から `bond_level` を計算する backend ロジックを追加
- `bond_level` に応じた返答開始待ち時間と表示速度を追加
- `bond_score >= 90` の時だけ、親しい語尾として「ぼんす」を一文だけ許可
- チャット完了時に `bond_score` を少し上げる
- 水やり検知時に `bond_score` を上げる
- 低水分検知時に `bond_score` を少し下げる
- チャットの system prompt を parenting chat 仕様へ寄せた
- WebSocket に `reply_start` を追加
- frontend で `reply_start.typing.delay_ms` と `speed` を受けて表示遅延・タイプライター表示を行う

## 変更ファイル

主な変更ファイル：

- `supabase/migrations/20260427000000_add_parenting_chat_state.sql`
- `backend/internal/models/bonsai_profile.go`
- `backend/internal/repository/bonsai_profile_repository.go`
- `backend/internal/service/bond_service.go`
- `backend/internal/service/body_state_service.go`
- `backend/internal/service/chat_prompt.go`
- `backend/internal/service/chat_service.go`
- `backend/internal/service/sensor_service.go`
- `backend/internal/realtime/hub.go`
- `backend/internal/api/handlers/chat_handler.go`
- `frontend/src/types/api.ts`
- `frontend/src/hooks/use-chat-socket.ts`

## あなたがやるべきこと

### 1. Supabase migration を適用する

ローカル Supabase を使っている場合：

```bash
supabase db reset
```

既存データを残したい環境では、今回追加した migration だけを本番/開発DBへ適用する。

対象 migration：

```text
supabase/migrations/20260427000000_add_parenting_chat_state.sql
```

追加されるカラム：

```text
bonsai_profiles.tone_mode
bonsai_profiles.bond_score
bonsai_profiles.bond_updated_at
```

### 2. backend を再起動する

```bash
cd backend
go run ./cmd/server
```

### 3. frontend を再起動する

```bash
cd frontend
npm run dev
```

### 4. 既存プロフィールを確認する

既存ユーザーの `bonsai_profiles` には migration により以下が入る。

```text
tone_mode = gentle
bond_score = 20
```

このため、最初はまだ「ぼんす」語尾は出ない。

### 5. 動作確認する

チャットで以下を確認する。

- 返答前に少し待ち時間がある
- 返答がタイプライター風に表示される
- 返答にセンサー数値が直接出にくい
- 雑談に短く返る
- `bond_score` が低い間は「ぼんす」が出ない

水やりイベント確認：

```bash
curl -s -X POST http://127.0.0.1:8080/api/sensors \
  -H "Content-Type: application/json" \
  -H "X-Sensor-Token: $SENSOR_API_TOKEN" \
  -H "X-Sensor-User-ID: <Supabase user id>" \
  --data '{
    "device_id":"mock-device-001",
    "recorded_at":"2026-04-27T10:30:00+09:00",
    "moisture_percent":70,
    "light_lux":12000,
    "temperature_celsius":24.3
  }'
```

水分が短時間で大きく上がると、既存の `watering_detected` が発火し、`bond_score` が上がる。

## ぼんす語尾の仕様

「ぼんす」は常時語尾ではなく、なじみ具合が最大に近い時だけ出る親しい癖として扱う。

条件：

- `bond_score >= 90`
- `bond_level = trusted`
- `tone_mode != classical`
- 乾きや不調が強くない
- 1返答につき最大1文

禁止：

- 全文を「ぼんす」語尾にする
- 初期状態から「ぼんす」を使う
- 深刻な相談や不調時に使う

## 確認済み

実行済み：

```bash
cd backend && go test ./...
cd frontend && npm run lint
cd frontend && npm run build
```

結果：

- backend test 成功
- frontend lint 成功
- frontend build 成功

## 後続候補

- `tone_mode` をUIから変更できる設定画面を追加する
- `bond_score` を直接数値表示せず、「なじみ具合」として表示する
- `care_events` テーブルを作って手入れ履歴を正規化する
- `bonsai_growth_states` テーブルを作って成長状態を保存する
- `bond_score` の日次上限をDB側でも保証する
