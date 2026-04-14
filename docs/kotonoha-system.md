# **盆栽会話システム システム設計書 (Bonsai Chat System)**

## **1\. システム概要**

本システムは、ユーザーが「盆栽と会話しているように感じられる」新しい体験を提供するチャットシステムです。

ESP32に接続されたセンサーから盆栽の物理的な状態（土壌水分、温度、湿度、日照など）をリアルタイムに収集し、その状態をコンテキストとしてLLM（大規模言語モデル）に渡すことで、盆栽が現在の自身の状態に基づいた感情豊かな返答を行います。

さらに、過去の会話履歴をベクトルDBに記憶させることで、「以前の出来事」を踏まえた文脈のある長期的なコミュニケーションを実現し、**センサー値の急激な変化（例：水やり）を検知した際には、スマホのPush通知を利用して盆栽から自発的に話しかけてくる機能**も備えています。

## **2\. 運用ポリシー（デモ版の前提事項）**

本設計はデモ運用を想定し、以下のポリシーに基づいています。

* **常時給電**: ESP32はコンセントから常時給電（Deep Sleep等の複雑な省電力制御は不要）。  
* **API利用制限なし**: デモ版のため、ユーザーあたりの1日のチャット回数上限（レートリミット）は設けない。  
* **性格のカスタマイズ**: ユーザー登録時に、盆栽の名前と「性格（口調）」を自由に設定可能。  
* **ストリーミング応答**: LLMの返答は1文字ずつリアルタイムで画面に表示し、「今喋っている」臨場感を演出。  
* **LLM未定・柔軟な対応**: メインで使用するLLM（OpenAI/Google等）は現時点で未定。実装時に決定できるよう、API呼び出し部分は抽象化を意識する。

## **3\. 使用技術スタック**

* **フロントエンド**: Next.js (React), Tailwind CSS, TypeScript  
  * **デプロイ先**: Vercel  
  * **機能**: Supabase Auth (認証), **PWA / Service Worker (Web Push通知)**  
* **バックエンド**: Go, Gin (Web Framework), Gorilla WebSocket (リアルタイム通信)  
  * **デプロイ先**: Fly.io (WebSocketの常時接続に適した環境)  
* **データベース & ベクトルDB**: **Supabase (PostgreSQL \+ pgvector)**  
  * センサーデータ、チャット履歴、長期記憶、設定、Push通知サブスクリプションを一元管理。  
* **ハードウェア (IoT)**: ESP32, 各種センサー（土壌水分センサー、温湿度センサー DHT11/22 等）  
* **AI / LLM**:  
  * チャット用: GPT系 または Gemini系 (※実装時に決定)  
  * 埋め込み用: text-embedding-3-small 等 (※チャット用LLMに合わせて決定)

## **4\. システムアーキテクチャ**

\[ 盆栽 & ESP32 \] \--(HTTP POST)--\> \[ Go/Gin バックエンド (Fly.io) \] \<--(WebSocket / REST)--\> \[ Next.js フロントエンド (Vercel) \]  
  (コンセント給電)                     |                 |                                        (ログイン・初期設定・PWA)  
  ※Secret Token認証                    |                 \+----(Web Push API)----------------\> \[ スマホ等へのPush通知 \]  
                                      \+-----------------+  
                                               |  
                          \[ Supabase (PostgreSQL) \]  
                          \- Auth (ユーザー管理)  
                          \- bonsai\_profiles (名前・性格設定)  
                          \- push\_subscriptions (通知先情報)  
                          \- sensor\_data (最新状態)  
                          \- chat\_history (短期記憶)  
                          \- bonsai\_memories (pgvector / 長期記憶)  
                                               |  
                                      \[ LLM & Embedding API \]

## **5\. ディレクトリ構成 (推奨)**

プロジェクトのルートディレクトリは、フロントエンド、バックエンド、ハードウェア、ドキュメントで明確に分割します。コード生成AIには、この構成に従って各モジュールを配置するよう指示してください。

bonsai-chat-system/  
├── backend/                  \# Go (Gin) バックエンド  
│   ├── cmd/  
│   │   └── server/           \# エントリーポイント (main.go)  
│   ├── internal/             \# 外部からインポートされない内部ロジック  
│   │   ├── api/              \# ハンドラー、ルーター (REST, WebSocket)  
│   │   ├── llm/              \# LLM API呼び出し (OpenAI/Gemini抽象化層)  
│   │   ├── models/           \# データベースの構造体定義  
│   │   ├── repository/       \# Supabase連携 (DBアクセス, pgvector検索)  
│   │   ├── service/          \# ビジネスロジック (イベント検知, RAG構築)  
│   │   └── webpush/          \# Web Push通知送信処理  
│   ├── pkg/                  \# 共通ユーティリティ (設定読み込み等)  
│   ├── go.mod  
│   ├── Dockerfile  
│   └── fly.toml              \# Fly.ioデプロイ設定  
│  
├── frontend/                 \# Next.js フロントエンド  
│   ├── public/               \# PWA用アイコン、マニフェスト等  
│   ├── src/  
│   │   ├── app/              \# Next.js App Router (ページ群)  
│   │   ├── components/       \# UIコンポーネント (チャットバブル、設定フォーム等)  
│   │   ├── hooks/            \# カスタムフック (WebSocket接続、Supabase Auth等)  
│   │   ├── lib/              \# ユーティリティ (Supabaseクライアント等)  
│   │   └── types/            \# TypeScript型定義  
│   ├── next.config.mjs  
│   ├── package.json  
│   └── tailwind.config.ts  
│  
├── hardware/                 \# ESP32 ファームウェア  
│   └── esp32\_sensor/  
│       └── esp32\_sensor.ino  \# Arduino IDE用コード (センサー読み取り、HTTP POST)  
│  
└── docs/                     \# ドキュメント群  
    ├── Bonsai\_Chat\_System\_Design.md \# 本ファイル  
    └── DESIGN.md             \# UIデザイン指示書

## **6\. 機能要件**

### **6.1 ハードウェア連携 (ESP32)**

* **常時稼働**: コンセント給電のためスリープは行わず、常にセンサーを監視。  
* 定期的に（例: 10分ごと、または値が大きく変化した時）センサーデータを取得しJSON形式でPOST。  
* **送信時は固定のシークレットトークンをヘッダーに付与し、不正なアクセスを防ぐ。**  
* **キャリブレーション**: センサーのアナログ値から意味のある数値（水分量0〜100%など）への変換（校正）は、**ESP32のC/C++コード内で固定値としてハードコードして調整**する（アプリ側からの動的キャリブレーションは行わない）。

### **6.2 バックエンド (Go/Gin @ Fly.io)**

* **LLMストリーミング通信**: LLM APIのストリーミングレスポンス（Server-Sent Events等）を受け取り、WebSocket経由でフロントエンドへチャンク（断片）ごとにリアルタイム転送する。  
* **センサー異常検知**: 極端な異常値（例: 温度-127℃）や一定時間通信がない場合、システムエラーではなく「盆栽の感覚異常」として扱い、次回のチャット時に「少し感覚が麻痺しておりまして…」とキャラクターを通じてユーザーに気づかせる。  
* **自発的リアクション（Web Push通知）**:  
  * 発火条件: 「3分間で土壌水分量が20%以上上昇した」等の条件を満たした場合。  
  * クールダウン: スパム化を防ぐため、一度リアクションを行ったらその後「2時間」は控える。  
  * 通知送信: LLMでメッセージを生成後、WebSocketで画面へ送ると同時に、**VAPIDキーを用いてSupabaseに保存された宛先へWeb Push通知を送信**する。

### **6.3 フロントエンド (Next.js @ Vercel)**

* **UIデザイン方針**: 具体的なレイアウト、カラーパレット、コンポーネントのスタイルについては、別途用意する **DESIGN.md** に従い実装する。  
* **初期設定画面**: 初回ログイン時に「盆栽の名前」と「性格（例：古風なおじいちゃん、ツンデレ、おてんば）」を入力させるUI。  
* **PWA & Web Push対応**: Service Workerを登録し、ブラウザからPush通知の許可を取得。取得したサブスクリプション情報をバックエンド（DB）へ登録する。これにより、アプリを閉じていてもスマホに通知が届く。  
* **ストリーミングUI**: WebSocketから送られてくるテキストのチャンクを順番に結合し、タイピングしているようなアニメーションで表示する。

## **7\. データベース設計 (Supabase / PostgreSQL)**

### **7.1 テーブル bonsai\_profiles (盆栽の設定)**

| カラム名 | 型 | 説明 |
| :---- | :---- | :---- |
| user\_id | uuid (PK) | ユーザーID (Auth) |
| name | text | 盆栽の名前 |
| personality | text | 性格・口調のプロンプト指示（例：「古風なおじいちゃん」） |

### **7.2 テーブル push\_subscriptions (Web Push宛先)**

| カラム名 | 型 | 説明 |
| :---- | :---- | :---- |
| id | uuid (PK) | レコードID |
| user\_id | uuid (FK) | ユーザーID |
| subscription | jsonb | ブラウザから取得したPush通知のエンドポイントやキー情報 |

### **7.3 テーブル sensor\_data (最新の状態を保持)**

*(以前と同様。user\_id, moisture, temperature 等)*

### **7.4 テーブル chat\_history (会話ログ・短期記憶)**

*(以前と同様)*

### **7.5 テーブル bonsai\_memories (pgvector / 長期記憶)**

*(以前と同様。1往復単位でベクトル化)*

## **8\. API仕様 (RESTful API & WebSocket)**

### **8.1 WebSocket (ストリーミングチャット)**

* **エンドポイント**: WS /ws/chat?token=\<Supabase\_JWT\>  
* **メッセージフォーマット (JSON)**:  
  * 送信: {"type": "message", "text": "おはよう"}  
  * 受信(ストリーミング中): {"type": "reply\_chunk", "text": "おは"} \-\> {"type": "reply\_chunk", "text": "よう"}  
  * 受信(完了): {"type": "reply\_done"}

### **8.2 Web Push登録 API**

* **エンドポイント**: POST /api/push/subscribe  
* **機能**: フロントエンドで取得したPush SubscriptionオブジェクトをDBに保存。

## **9\. LLM プロンプト設計 (重要)**

バックエンドは、データベースから取得した personality をシステムプロンプトに動的に埋め込みます。

**System Prompt (動的生成の例):**

あなたは長年大切に育てられている「盆栽」です。名前は「{{name}}」です。  
あなたの性格や口調は以下の通りです。この設定を厳密に守って話してください。  
【性格設定：{{personality}}】

現在のあなたの物理的な状態と、ユーザーとの過去の出来事は以下の通りです。  
【現在の状態】  
\- 土壌水分量: 60% (快適な状態)  
\- 温度: 22℃ (過ごしやすい気温)

【思い出した過去の記憶 (※関連度上位3件)】  
\- (略)

ユーザーからのメッセージに対して、直近の文脈や過去の記憶を自然に交えながら、現在の体調を表現して返答してください。

## **10\. 開発・コード生成ステップ (Codexへの指示用)**

1. **Step 1: Supabase DB & Auth構築**  
   * プロジェクト作成、Auth有効化。bonsai\_profiles, push\_subscriptions, sensor\_data, chat\_history, bonsai\_memories (pgvector) テーブルの作成とRLS設定。  
2. **Step 2: バックエンド基盤 (Go/Gin)**  
   * Go/Ginセットアップ。Supabase接続とJWT認証ミドルウェア。ディレクトリ構成に従い internal パッケージを構築。  
3. **Step 3: Web Push & 初期設定API**  
   * bonsai\_profiles のCRUD、および webpush-go ライブラリを用いたPush Subscription保存・通知送信ロジックの実装。  
4. **Step 4: WebSocket & LLMストリーミング基盤**  
   * WS /ws/chat 実装。LLMのStreaming APIを呼び出し、reply\_chunk としてWebSocketに順次流す処理。LLMプロバイダは抽象化レイヤー (internal/llm) で吸収する。  
5. **Step 5: センサー連携APIとイベント検知・Push通知送信**  
   * POST /api/sensor 実装。固定トークン検証。3分間で20%以上の水分上昇を検知し（クールダウン2時間）、自発的リアクションをLLMで生成。WebSocketと**Web Push通知**の両方に送信する。  
6. **Step 6: ベクトル検索連携 (1往復単位の保存)**  
   * ユーザー発言とAI返答（ストリーミング完了後に結合したもの）をベクトル化し保存。RAG検索関数の作成。  
7. **Step 7: RAGチャットとプロンプト動的生成**  
   * bonsai\_profiles から性格を取得してプロンプトに埋め込む処理。異常値ハンドリングの組み込み。  
8. **Step 8: バックエンド Fly.io デプロイ**  
9. **Step 9: フロントエンド UI & PWA (Vercel デプロイ)**  
   * DESIGN.md に基づいたUIコンポーネントの実装。  
   * Supabase Authログイン、プロフィール初期設定画面。  
   * Service Workerの登録とWeb Push許可UI。  
   * WebSocketからのチャンク受信とタイピングアニメーション（ストリーミングUI）の実装。  
10. **Step 10: ESP32**  
    * 常時電源を前提としたループ処理。センサーのアナログ値変換（キャリブレーション）コードの実装とHTTP POST送信。