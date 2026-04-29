# TTS Implementation Plan

## 目的

Kotonoha のチャット応答を画面表示だけでなく音声でも届けるため、ブラウザ標準の Web Speech API (`speechSynthesis`) を使って、盆栽の返答を日本語で読み上げる。

この計画は TTS の実装範囲に限定する。LLM、WebSocket、DB、プロンプト、会話履歴の保存形式は変更しない。

---

## 現状

### 実装済み

- フロントエンドは Next.js / React。
- チャット UI は `frontend/src/components/chat-panel.tsx`。
- WebSocket 接続とストリーミング表示は `frontend/src/hooks/use-chat-socket.ts`。
- バックエンドは `reply_start`、`reply_chunk`、`reply_done` を返す。
- フロントエンドでは `reply_chunk` を文字キューに入れ、タイプライター風に表示している。
- `reply_done` 受信後も、表示キューが空になるまで assistant の返答は完了扱いにならない。

### 未実装

- assistant 返答の読み上げ。
- 音声 ON/OFF。
- 読み上げ停止。
- ブラウザが音声合成に非対応な場合の扱い。

---

## 方針

### バックエンドは変更しない

TTS は表示済みテキストをブラウザで読み上げるだけなので、Go backend、Gemini、Supabase には変更を入れない。

### ストリーミング中には読まない

`reply_chunk` ごとに読み上げると、発話が途切れて不自然になる。MVP では assistant の全文表示が完了したあとに一度だけ読み上げる。

### ユーザー操作で有効化する

Safari / iOS などでは、音声再生がユーザー操作なしに始まらない場合がある。初期状態は OFF とし、ユーザーが「音声」トグルを ON にした後だけ読み上げる。

### TTS はフロントエンドの専用 hook に分離する

`chat-panel.tsx` に直接 `speechSynthesis` の制御を増やすと UI と副作用が混ざるため、専用 hook を追加する。

追加候補:

```text
frontend/src/hooks/use-speech-synthesis.ts
```

---

## MVPスコープ

- assistant の最新返答を日本語で読み上げる。
- 読み上げは全文表示完了後に開始する。
- 音声 ON/OFF を切り替えられる。
- 読み上げ中に停止できる。
- 新しい assistant 返答を読む前に、前回の読み上げを停止する。
- センサーイベント (`source=sensor_event`, role=`event`) は MVP では読み上げない。
- 会話履歴読み込み時の過去メッセージは読み上げない。
- ブラウザ非対応時は UI を無効化し、アプリ全体は通常通り動かす。

後回し:

- 声の選択 UI。
- 音量、ピッチ、速度の詳細設定 UI。
- 英語読み上げ。
- センサーイベント通知の自動読み上げ。
- 返答文を文単位に分割した逐次読み上げ。
- サーバーサイド TTS。

---

## UX設計

### 配置

`chat-panel.tsx` のチャットヘッダー右側、または composer footer 付近に音声操作を置く。

MVP では以下の2操作でよい。

- 音声 ON/OFF トグル
- 停止ボタン

表示例:

```text
音声: ON
停止
```

既存 UI の密度が高いため、ラベルは短くする。

### 状態

UI で扱う状態:

- `supported`: ブラウザが `speechSynthesis` と `SpeechSynthesisUtterance` に対応しているか
- `enabled`: 読み上げを有効にしているか
- `speaking`: 現在読み上げ中か
- `error`: 読み上げ開始に失敗した場合の短いメッセージ

### 読み上げ対象

読み上げ対象は、新規に生成された assistant 返答のみ。

読み上げないもの:

- user メッセージ
- system メッセージ
- event メッセージ
- 初期プレースホルダー文
- 履歴取得で復元された assistant メッセージ
- 空文字の pending assistant

---

## Frontend 変更

## 1. TTS hook を追加

対象:

```text
frontend/src/hooks/use-speech-synthesis.ts
```

責務:

- Web Speech API 対応状況を判定する。
- `speak(text)` で読み上げを開始する。
- `stop()` で読み上げを停止する。
- `enabled` を保持する。
- unmount 時に `speechSynthesis.cancel()` する。

想定 API:

```ts
type SpeechSynthesisState = {
  supported: boolean;
  enabled: boolean;
  speaking: boolean;
  errorMessage: string;
  setEnabled: (enabled: boolean) => void;
  speak: (text: string) => void;
  stop: () => void;
};
```

読み上げ設定:

```ts
const utterance = new SpeechSynthesisUtterance(text);
utterance.lang = 'ja-JP';
utterance.rate = 0.8;
speechSynthesis.cancel();
speechSynthesis.speak(utterance);
```

補足:

- `text.trim()` が空なら何もしない。
- `enabled === false` なら何もしない。
- `supported === false` なら何もしない。
- `onstart`, `onend`, `onerror` で `speaking` を更新する。

## 2. useChatSocket から「完成した新規 assistant 返答」を受け取れるようにする

対象:

```text
frontend/src/hooks/use-chat-socket.ts
```

現状の `finishAssistantIfReady()` は、`reply_done` 受信済みかつ文字キューが空になった時点で assistant 返答を完了させている。このタイミングが TTS 開始に適している。

追加案:

```ts
export function useChatSocket(
  token: string,
  enabled: boolean,
  options?: {
    onAssistantReplyComplete?: (text: string) => void;
  },
)
```

`finishAssistantIfReady()` 内で、完了した assistant メッセージ本文を取得して callback する。

注意:

- callback は新規応答に対して一度だけ呼ぶ。
- 履歴読み込み時には呼ばない。
- error 時や空返答では呼ばない。
- sensor event の `reply_done` では呼ばない。

実装上の候補:

- `activeAssistantIdRef.current` を使って、完了直前に `messages` から対象メッセージを探す。
- React state の非同期更新に注意し、完了本文を別の ref に蓄積してもよい。
- 既存の表示キュー処理を壊さないことを優先する。

より堅い案:

```ts
const assistantFullTextRef = useRef('');
```

- `reply_start` で空にする。
- `reply_chunk` 受信時に chunk を追加する。
- `finishAssistantIfReady()` で `assistantFullTextRef.current` を callback へ渡す。
- callback 後に空にする。

この案なら、表示用 state の反映タイミングに依存しない。

## 3. ChatPanel に音声 UI と callback を接続する

対象:

```text
frontend/src/components/chat-panel.tsx
```

変更:

- `useSpeechSynthesis()` を呼ぶ。
- `useChatSocket(token, true, { onAssistantReplyComplete })` の形で callback を渡す。
- callback 内で `speech.speak(text)` を呼ぶ。
- ヘッダーまたは composer footer に音声トグルと停止ボタンを置く。

例:

```ts
const speech = useSpeechSynthesis();

const { messages, status, errorMessage, sendMessage } = useChatSocket(token, true, {
  onAssistantReplyComplete: (text) => speech.speak(text),
});
```

UI の条件:

- `speech.supported === false` の場合は音声トグルを disabled にする。
- `speech.enabled === false` の場合は停止ボタンを disabled にする。
- `speech.speaking === true` の場合だけ停止ボタンを有効にする。

## 4. CSS を追加

対象:

```text
frontend/src/app/globals.css
```

追加候補:

- `.k-voice-controls`
- `.k-voice-toggle`
- `.k-voice-stop-btn`

既存の `k-lang-switcher` や `k-esp-refresh-btn` と見た目を合わせる。

---

## エラー処理

### ブラウザ非対応

`typeof window === 'undefined'` または `speechSynthesis` が存在しない場合:

- `supported=false`
- 読み上げ UI は disabled
- チャット機能は通常通り動作

### 読み上げ失敗

`utterance.onerror` で `errorMessage` を設定する。

ただし、TTS 失敗はチャット送受信の失敗ではないため、既存の WebSocket エラー表示とは分ける。

### 連続送信

新しい assistant 返答を読み上げる前に必ず `speechSynthesis.cancel()` する。

これにより、前回の読み上げが残ったまま次の返答と重なることを防ぐ。

---

## テスト計画

### 手動確認

1. 音声 OFF の状態でチャットを送る。
   - 文字表示のみで、読み上げされない。
2. 音声 ON にしてチャットを送る。
   - assistant の全文表示後に読み上げが始まる。
3. 読み上げ中に停止を押す。
   - 音声が止まる。
4. 読み上げ中に次のメッセージを送る。
   - 前回音声が止まり、新しい返答だけ読む。
5. ページを離れる。
   - 読み上げが止まる。
6. 履歴がある状態でページを開く。
   - 過去の assistant メッセージを勝手に読まない。
7. センサーイベントが流れる。
   - MVP では読み上げない。

### 自動テスト候補

フロントエンドにテスト基盤がある場合:

- `use-speech-synthesis` の `supported=false` 分岐。
- `enabled=false` で `speak()` が何もしないこと。
- `speak()` 時に `speechSynthesis.cancel()` の後で `speak()` が呼ばれること。
- `useChatSocket` の `onAssistantReplyComplete` が `reply_done` とキュー消化後に一度だけ呼ばれること。

---

## 実装順

1. `use-speech-synthesis.ts` を追加する。
2. `useChatSocket` に `onAssistantReplyComplete` callback を追加する。
3. `ChatPanel` で callback と TTS hook を接続する。
4. `ChatPanel` に音声 ON/OFF と停止ボタンを追加する。
5. `globals.css` に音声 UI のスタイルを追加する。
6. ブラウザで手動確認する。

---

## 完了条件

- 音声 ON の時、assistant の新規返答が全文表示後に日本語で読み上げられる。
- 音声 OFF の時、読み上げされない。
- 停止ボタンで読み上げを止められる。
- 履歴読み込み時に過去メッセージを読み上げない。
- TTS 非対応ブラウザでもチャット機能が壊れない。
- バックエンド変更なしで実現できている。

