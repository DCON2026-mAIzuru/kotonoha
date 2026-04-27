# ソフトウェア進捗

### 1. LLM 応答まわりの改善

- `backend/internal/llm/streamer.go`
- `backend/internal/llm/gemini_streamer.go`
- `backend/internal/llm/streamer_test.go`

主な進捗:

- Gemini ストリーミング処理の改善
- LLM ストリーマーの共通化・整理
- ストリーミング処理のテスト追加
-

### 2. フロントエンド UI の調整

- `frontend/src/components/kotonoha-app.tsx`
- `frontend/src/components/chat-panel.tsx`
- `frontend/src/app/globals.css`
- `frontend/src/components/pwa-install-button.tsx`

主な進捗:

- チャット UI の見た目調整
- ホーム画面側の表示改善
- PWA インストール導線の調整

### 3. 設定まわりの更新

- `backend/pkg/config/config.go`
- `backend/cmd/server/main.go`
- `README.md`

主な進捗:

- LLM 関連の設定値整理
- サーバー起動時の設定反映を更新
- README の技術説明を最新構成に追従
- 盆栽の来歴や、前の飼い主との思い出を話せるようにする
- 単なるセンサー応答ではなく、「長く生きてきた盆栽」として過去を語る人格を持たせる
- 現在のユーザーとの会話記憶だけでなく、初期設定されたバックストーリーを会話に混ぜる（実装中）
- 例: 「前の持ち主は朝に水をくれる人だった」「昔は別の庭で育っていた」などを、必要なときに自然に話す(実装中)

## 次に見るべきところ

- センサー固定値の扱いを永続化するか、一時メモリのままにするか
- 盆栽の来歴・前の飼い主の思い出を、プロフィール項目として実装するかどうか
- 低水分イベントと水やりイベントの閾値調整
- フロントのセンサー表示と会話 UI の情報量バランス調整
- ESP 実機センサー値への置き換えと最終接続確認
