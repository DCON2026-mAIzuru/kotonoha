# BONSAI Parenting Chat Implementation Plan

## 目的

現状の Kotonoha 実装に、`BONSAI-parenting-chat-prompt.md` の「雑談」「世話」「育っている感覚」「なじみ具合」を組み込むための実装計画である。

この計画はゼロからの設計ではない。すでに実装済みの Go/Gin backend、Next.js frontend、Supabase schema、WebSocket streaming、Gemini 抽象化、RAG、センサーイベント、Push 通知を前提に、差分として何を追加・置換するかを整理する。

---

## 現状把握

### 実装済み

- Supabase Auth + Google ログイン
- `bonsai_profiles`
  - `name`
  - `personality`
  - `backstory`
- `sensor_data`
  - 最新センサー状態をユーザー単位で保持
  - `moisture_percent`
  - `light_lux`
  - `temperature_celsius`
  - `sensor_status`
- `chat_history`
  - `user` / `assistant` / `system` / `event`
  - 1発話単位で保存
- `bonsai_memories`
  - 1往復単位で長期記憶化
  - pgvector 3072次元
- WebSocket chat
  - `reply_chunk`
  - `reply_done`
  - `error`
- センサーイベント
  - `watering_detected`
  - `low_moisture_detected`
  - 2時間クールダウン
  - WebSocket broadcast
  - Web Push 通知
- フロントエンド
  - チャットUI
  - センサー表示
  - 水分手動固定
  - Push/PWA
  - WebSocket hook

### まだ未実装

- `bond_score`
- `bond_level`
- `tone_mode`
- `reply_start`
- 返答開始待ち時間
- タイプライター速度制御
- `care_events` 専用テーブル
- `growth_state`
- `BONSAI-parenting-chat-prompt.md` に沿った system prompt

---

## 方針

### 既存を活かす

現状の `chat_history` は `event` role を持ち、センサーイベント履歴も保存している。MVPでは新規 `care_events` テーブルを急いで作らず、まず `chat_history.role = event` と `sensor_snapshot` を手入れ履歴として利用する。

ただし、`bond_score` はプロフィールに属する長期状態なので `bonsai_profiles` に追加する。

### 体験の中心を変更する

現状の prompt は「明るくテンポよく」「ぼんす語尾」が中心である。子育て・雑談・世話の感覚を出すには、以下へ寄せる。

- センサー数値を直接読ませない
- 身体状態を自然言語化する
- 毎回「ぼんす」を強制しない
- `bond_score` が 90 以上に近づいた時だけ、親しい癖として「ぼんす」を少し使う
- `tone_mode` で話し方を制御する
- `bond_level` で距離感と表示速度を制御する
- `backstory` は毎回語らず、深いなじみ段階でだけ少し出す

---

## MVPスコープ

最初の完成ラインは以下。

- DBに `bond_score`, `bond_updated_at`, `tone_mode` を追加する
- Go backend で `BondService` を追加する
- Go backend で `PromptBuilder` 相当の関数群を整理する
- 現在の `buildChatSystemPrompt` を parenting chat prompt 仕様へ置き換える
- センサーイベントとチャット送信で `bond_score` を更新する
- WebSocket に `reply_start` を追加する
- frontend で `reply_start.typing.delay_ms` を受けて表示待ち時間を変える
- frontend で `reply_start.typing.speed` を受けてタイプライター表示速度を変える
- UIには英語の `bond_level` を出さない

後回し：

- `care_events` 専用テーブル
- `bonsai_growth_states` 専用テーブル
- なじみ具合の詳細UI
- 複数盆栽
- DNA生成フロー

---

## DB変更

### 新規マイグレーション

追加ファイル案：

```text
supabase/migrations/20260427000000_add_parenting_chat_state.sql
```

### bonsai_profiles 追加カラム

```sql
alter table public.bonsai_profiles
  add column if not exists tone_mode text not null default 'gentle',
  add column if not exists bond_score integer not null default 20,
  add column if not exists bond_updated_at timestamptz not null default timezone('utc', now());
```

制約：

```sql
alter table public.bonsai_profiles
  add constraint bonsai_profiles_tone_mode_check
  check (tone_mode in ('gentle', 'childlike', 'calm', 'classical'));

alter table public.bonsai_profiles
  add constraint bonsai_profiles_bond_score_check
  check (bond_score >= 0 and bond_score <= 100);
```

注意：

- 既存 `personality` は残す
- 既存 `backstory` は `bonsai_dna` の簡易版として扱う
- `bond_score` 初期値は 20 にする
- `tone_mode` 初期値は `gentle` にする

---

## Backend 変更

## 1. Models

対象：

```text
backend/internal/models/bonsai_profile.go
```

追加：

```go
ToneMode      string    `json:"tone_mode"`
BondScore     int       `json:"bond_score"`
BondUpdatedAt time.Time `json:"bond_updated_at"`
```

`UpsertBonsaiProfileInput` にも `ToneMode` を追加する。`bond_score` はプロフィール設定フォームから直接更新しない。

初期設定APIで `tone_mode` を未指定にする場合、service 層で `gentle` を補完する。

## 2. Repository

対象：

```text
backend/internal/repository/bonsai_profile_repository.go
```

変更：

- `select` に `tone_mode`, `bond_score`, `bond_updated_at` を追加
- `insert/update` に `tone_mode` を追加
- `UpdateBonsaiBondScore` を追加

追加メソッド案：

```go
UpdateBonsaiBondScore(ctx context.Context, userID string, score int) (*models.BonsaiProfile, error)
```

将来の同時更新を考えるなら、差分更新でもよい。

```go
ApplyBonsaiBondDelta(ctx context.Context, userID string, delta int, dailyLimit bool) (*models.BonsaiProfile, error)
```

MVPではアプリ側で clamp して `UpdateBonsaiBondScore` する。

## 3. BondService

新規：

```text
backend/internal/service/bond_service.go
backend/internal/service/bond_service_test.go
```

責務：

- `bond_score` から `bond_level` を返す
- イベント種別から `score_delta` を返す
- 0から100に丸める
- 1日の上昇/下降上限を守る
- `bond_state` prompt block を作る
- frontend へ渡す typing profile を作る

型案：

```go
type BondLevel string

const (
	BondLevelCautious BondLevel = "cautious"
	BondLevelAware    BondLevel = "aware"
	BondLevelFamiliar BondLevel = "familiar"
	BondLevelAttached BondLevel = "attached"
	BondLevelTrusted  BondLevel = "trusted"
)

type TypingProfile struct {
	DelayMS   int    `json:"delay_ms"`
	Speed     string `json:"speed"`
	BondLevel string `json:"bond_level"`
}
```

変換：

| score | level | delay |
|-------|-------|-------|
| 0-19 | cautious | 1200ms |
| 20-39 | aware | 900ms |
| 40-59 | familiar | 650ms |
| 60-79 | attached | 400ms |
| 80-100 | trusted | 250ms |

`sensor_status != normal` または水分がかなり低い時は `delay_ms` を +200 から +400 する。

追加ルール：

- `bond_score < 90` では「ぼんす」語尾を使わせない
- `bond_score >= 90` かつ `bond_level = trusted` の時だけ、「ぼんすよ」「ぼんすね」などを一文だけ許可する
- `classical` モードでは、`bond_score >= 90` でも原則使わない
- 乾きが強い時、不調時、深刻な相談時には使わない

## 4. ChatService

対象：

```text
backend/internal/service/chat_service.go
```

現状：

- `buildChatSystemPrompt` がこのファイル内にある
- センサー値を数値込みで prompt に渡している
- 「ぼんす」語尾を強く指定している
- RAG はすでに動く
- 会話後に `bonsai_memories` を作る

変更方針：

- `formatSensorPromptSection` を `buildBodyStatePromptSection` へ置き換える
- 数値直接表現を避ける
- `buildBondStatePromptSection` を追加する
- `buildToneModePromptSection` を追加する
- `formatRecentHistoryPromptSection` は利用継続
- `formatMemoryPromptSection` は利用継続
- `buildBackstoryPromptSection` は利用継続。ただし `trusted` 未満では強く出さない指示にする
- 「ぼんす」強制は削除する
- `bond_score >= 90` の時だけ、親しい語尾として一文まで許可する

新しい prompt 構造：

```text
{base_principle}

---

## この個体の樹格
{profile/backstory}

---

## 今日の身体
{body_state}

---

## 育ちの記録
{growth_state}

---

## 手入れの記録
{care_history}

---

## なじみ具合
{bond_state}

---

## 根に残る会話の欠片
{memory_fragments}

---

## 直近の会話
{dialogue_history}

---

## 今日の話し方
{tone_mode}
```

MVPでは `growth_state` は簡易生成にする。

例：

```text
最近の水分と会話から見る限り、朝の声には少し慣れてきている。
大きな成長報告はまだない。
```

## 5. Chat WebSocket

対象：

```text
backend/internal/api/handlers/chat_handler.go
backend/internal/realtime/hub.go
```

現状の `RealtimeMessage`：

```json
{ "type": "reply_chunk", "text": "..." }
{ "type": "reply_done" }
{ "type": "error", "message": "..." }
```

追加：

```json
{
  "type": "reply_start",
  "source": "chat",
  "typing": {
    "delay_ms": 400,
    "speed": "fast",
    "bond_level": "attached"
  }
}
```

実装案：

- `ChatService.StreamReply` の戻り値または callback を拡張する
- 低影響にするなら、`ChatService.GetTypingProfile(ctx, userID)` を handler から呼んで `reply_start` を先に送る
- その後、現状どおり `StreamReply` を呼ぶ

推奨：

```go
type StreamChatMeta struct {
	Typing TypingProfile `json:"typing"`
}

func (s *ChatService) PrepareStreamMeta(ctx context.Context, userID string) (*StreamChatMeta, error)
```

ただし prompt 構築でも profile/sensor を読むため、二重取得になる。MVPでは許容し、後で `StreamReply` 内に統合する。

## 6. SensorService

対象：

```text
backend/internal/service/sensor_service.go
```

現状：

- `watering_detected`
- `low_moisture_detected`
- event を `chat_history` に保存
- Push と WebSocket broadcast 済み

追加：

- `watering_detected` 発生時に `bond_score +2`
- `low_moisture_detected` 発生時に `bond_score -1`
- センサーが invalid/stale で下げすぎない
- 水分手動固定はデモ用なので、加点/減点への扱いを控えめにする

MVPでは `chat_history.event` を `care_history` として使うため、専用 `care_events` テーブルは作らない。

将来：

- `care_events` を作り、センサーイベントと会話イベントを正規化する
- `chat_history.event` はユーザーに見せる出来事ログとして残す

---

## Frontend 変更

## 1. Types

対象：

```text
frontend/src/types/api.ts
```

変更：

```ts
export interface TypingProfile {
  delay_ms: number;
  speed: 'slow' | 'normal' | 'fast';
  bond_level: 'cautious' | 'aware' | 'familiar' | 'attached' | 'trusted';
}

export interface RealtimeMessage {
  type: 'reply_start' | 'reply_chunk' | 'reply_done' | 'error';
  text?: string;
  message?: string;
  source?: string;
  event?: string;
  typing?: TypingProfile;
}
```

`BonsaiProfile` にも追加：

```ts
tone_mode: 'gentle' | 'childlike' | 'calm' | 'classical';
bond_score: number;
bond_updated_at: string;
```

## 2. useChatSocket

対象：

```text
frontend/src/hooks/use-chat-socket.ts
```

現状：

- 送信直後に空の assistant message を作る
- `reply_chunk` が来たらそのまま文字列連結
- pending 表示は `text === ''`

変更：

- `reply_start` を受ける
- `delay_ms` の間、pending indicator を維持
- チャンクを即表示せず、キューにためてタイプライター表示する
- `speed` に応じて flush 間隔を変える

実装の簡易案：

```ts
const chunkQueueRef = useRef<string[]>([]);
const typingProfileRef = useRef<TypingProfile | null>(null);
const typingStartedRef = useRef(false);
```

流れ：

1. 送信時に assistant placeholder を作る
2. `reply_start` で profile を保存
3. `delay_ms` 後にキュー消費開始
4. `reply_chunk` は queue に入れる
5. interval で数文字ずつ assistant text に追加
6. `reply_done` 後、queue が空になったら active id を解除

MVP簡略化：

- まずは `delay_ms` だけ実装
- タイプライター速度は次ステップでもよい

## 3. ChatPanel

対象：

```text
frontend/src/components/chat-panel.tsx
```

現状：

- センサー値を数値で表示
- 初期メッセージが明るい「ぼんす」調
- なじみ具合表示なし

変更：

- 初期メッセージを `gentle` 寄りにする
- `bond_score` 表示はMVPでは不要
- 表示するなら「なじみ具合: 手になじむ」のように出す
- pending 文言を `言葉を選んでいます...` のまま利用してよい

初期メッセージ案：

```text
こんにちは。今日は鉢のそばで静かに聞いているよ。
水や光のことでも、ただの雑談でも、少しずつ覚えていくね。
```

## 4. Profile Setup

対象：

```text
frontend/src/components/profile-setup-form.tsx
frontend/src/lib/backend-api.ts
```

追加：

- `tone_mode` 選択
- 初期値 `gentle`

MVPでは必須ではない。未指定なら backend で補完する。

---

## Prompt 置換計画

### 現状 prompt の課題

現在の `buildChatSystemPrompt` はデモとしては明るいが、今回の体験とはズレる。

課題：

- センサー値を `土壌水分量 40.0%` のように直接渡している
- 「ぼんす」語尾を強制している
- 水分状態への反応がかなり強い
- 雑談と子育て感より、キャラクター性が前に出る

### 新 prompt の要点

- 1から4文
- 雑談では1から2文
- センサー数値を直接言わない
- 世話に小さく反応する
- `bond_level` が低い時は距離を取る
- `bond_level` が高い時は語尾が柔らかくなる
- `bond_score >= 90` でだけ「ぼんす」語尾を一文だけ使える
- 高bondでも依存しない
- `backstory` は必要な時だけ出す

### 関数分割

`chat_service.go` が大きくなっているため、以下に分ける。

```text
backend/internal/service/chat_prompt.go
backend/internal/service/body_state_service.go
backend/internal/service/bond_service.go
```

ただし、初回実装では `chat_service.go` 内の関数追加でもよい。テストしやすさを優先するなら分割する。

---

## care_history のMVP実装

専用テーブルなしで、既存 `chat_history` から作る。

対象イベント：

- `role = event`
- `sensor_snapshot.event = watering_detected`
- `sensor_snapshot.event = low_moisture_detected`

表示変換：

| event | care_history 文 |
|-------|-----------------|
| watering_detected | 水を与えられた |
| low_moisture_detected | 少し乾きに気づいた |

Repository 追加案：

```go
ListRecentChatHistoryEventsByUserID(ctx, userID string, limit int) ([]models.ChatHistoryEntry, error)
```

既存 `ListRecentChatHistoryByUserID` を流用して service 側で role filter してもよいが、DB側で絞る方が安定する。

---

## growth_state のMVP実装

専用テーブルは後回し。最初は `bond_level` とセンサー状態から生成する。

例：

```text
まだ大きな成長の変化はない。
ただし、朝の声と水の時間には少し慣れてきている。
```

`trusted` 以上：

```text
最近は水の時間に体が合ってきている。
葉の先が光に向くまでの迷いが少なくなっている。
```

乾きが続く時：

```text
ここ数回は乾きが先に立っており、成長の話は控えめにする。
```

---

## API / WebSocket 仕様変更

### RealtimeMessage

追加後：

```json
{
  "type": "reply_start",
  "source": "chat",
  "typing": {
    "delay_ms": 650,
    "speed": "normal",
    "bond_level": "familiar"
  }
}
```

既存互換：

- 既存 frontend は `reply_start` を無視できるようにする
- 新 frontend は `reply_start` が来ない場合、従来通り即時表示する

### REST profile

profile response に追加：

```json
{
  "tone_mode": "gentle",
  "bond_score": 20,
  "bond_updated_at": "2026-04-27T00:00:00Z"
}
```

UIでは `bond_score` を主役にしない。

---

## 実装順序

### Phase 1: DBと型

1. Supabase migration を追加
2. `models.BonsaiProfile` を更新
3. `BonsaiProfileRepository` を更新
4. frontend `BonsaiProfile` 型を更新
5. profile API の既存テストまたは手動確認

### Phase 2: BondService

1. `bond_service.go` を追加
2. score -> level 変換を実装
3. typing profile を実装
4. score delta を実装
5. unit test を追加

### Phase 3: Prompt 差し替え

1. `body_state` 生成を追加
2. `bond_state` 生成を追加
3. `tone_mode` prompt を追加
4. `care_history` を既存 `chat_history.event` から生成
5. `buildChatSystemPrompt` を置き換え
6. 既存 `chat_service_test.go` を新promptに合わせて更新

### Phase 4: bond_score 更新

1. チャット完了時に `talked +1` を適用
2. 水やり検知時に `+2`
3. 低水分検知時に `-1`
4. 1日上限はMVPでは軽く実装、厳密化は後続
5. `sensor_service_test.go` を更新

### Phase 5: reply_start

1. `realtime.Message` に `Typing` を追加
2. `RealtimeMessage` frontend 型を更新
3. `ChatHandler` で `reply_start` を送る
4. `useChatSocket` で `reply_start` を処理
5. 最初は `delay_ms` のみ反映
6. 次にタイプライター速度を反映

### Phase 6: UI調整

1. 初期メッセージを parenting chat 寄りに変更
2. クイックプロンプトを雑談・世話寄りに調整
3. 必要なら `tone_mode` 設定を追加
4. なじみ具合の表示は後続

---

## テスト計画

### Go Unit Test

追加：

- `BondLevelFromScore`
- `ClampBondScore`
- `TypingProfileForBond`
- `BuildBondStatePromptSection`
- `BuildBodyStatePromptSection`

更新：

- `TestStreamReplyBuildsDynamicPrompt`
  - `bond_state`
  - `今日の身体`
  - `今日の話し方`
  - センサー数値を直接出さない方針
- `sensor_service_test`
  - watering で bond 加点
  - low moisture で bond 減点

### Frontend

最低限の手動確認：

- `reply_start` が来ても壊れない
- `delay_ms` 中は pending indicator が出る
- `reply_chunk` が遅れて表示される
- `reply_done` 後に入力可能になる
- `bond_level` の英語名が画面に直接出ない

### Integration

ローカル確認：

1. `go test ./...`
2. frontend lint/build
3. mock sensor POST
4. WebSocket chat
5. 水やりイベント
6. 低水分イベント

---

## リスク

| リスク | 対策 |
|--------|------|
| 既存の「ぼんす」キャラから急に変わる | `personality` に書かれている場合だけ残す。system側では強制しない |
| `reply_start` 追加で既存WebSocketが壊れる | frontend側で未知typeを無視できる実装にする |
| bond更新でDBアクセスが増える | MVPではチャット完了時とイベント時だけ更新する |
| センサー誤差でbondが下がる | 減点は小さく、1日の下降上限を設ける |
| promptが長くなる | RAG件数と履歴件数は現状どおり少なめに保つ |
| なじみ具合が恋愛/ゲームに見える | UIでは数値を主役にせず「手になじむ」などの状態文にする |
| 「ぼんす」が出すぎて軽くなる | `bond_score >= 90`、1返答1文まで、`classical` と不調時は禁止にする |

---

## 完了条件

MVP完了条件：

- `bonsai_profiles` に `tone_mode`, `bond_score`, `bond_updated_at` がある
- チャット時の system prompt に `body_state`, `bond_state`, `tone_mode`, `care_history` が入る
- センサー値がLLM返答に数値として直接出にくくなる
- 水やりイベントで `bond_score` が上がる
- 低水分イベントで `bond_score` が少し下がる
- WebSocketで `reply_start` が送られる
- frontendで `delay_ms` が反映される
- 高bondほど返答開始が少し早くなる
- `bond_score >= 90` の時だけ、返答の一文に「ぼんす」語尾が自然に出る
- LLM返答に「好感度」「スコア」「レベル」が出ない
- 既存の RAG、Push、センサー表示が壊れていない

---

## 後続でやること

- `care_events` テーブルを作ってイベント履歴を正規化する
- `bonsai_growth_states` テーブルを作って成長状態を保存する
- `bond_score` の日次上限をDB側でも保証する
- `tone_mode` の設定UIを整える
- `BONSAI-DNA.md` を初期設定フローに組み込む
- `backstory` を `bonsai_dna` に発展させる
- なじみ具合UIを追加する
- Push通知文も parenting chat prompt に寄せる
