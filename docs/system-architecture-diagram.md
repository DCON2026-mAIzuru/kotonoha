# システム構成図

`Kotonoha` の主要コンポーネントとデータの流れをまとめた構成図です。  
DCON のポスターや口頭説明では、まずこの図をベースに `センサー入力 -> 文脈生成 -> 応答/通知` の流れを説明すると通しやすいです。

## 画像版

![Kotonoha システム構成図](./system-architecture-diagram.svg)

## 全体構成

```mermaid
flowchart LR
    user[ユーザー]
    browser[ブラウザ / PWA<br/>Next.js]
    sw[Service Worker<br/>Web Push受信]
    esp[ESP32 + 各種センサー<br/>水分・照度・温度]

    subgraph backend["バックエンド (Go / Gin on Fly.io)"]
        api[REST API]
        ws[WebSocket]
        chat[Chat Service]
        sensor[Sensor Service]
        push[Push Sender]
        rag[RAG / Prompt Builder]
    end

    subgraph supabase["Supabase"]
        auth[Auth]
        db[(PostgreSQL)]
        vector[(pgvector)]
    end

    llm[Gemini API<br/>応答生成 / 埋め込み]
    pushsvc[Push Service]

    user -->|操作・会話| browser
    browser -->|ログイン| auth
    browser -->|REST API| api
    browser -->|WebSocket| ws
    browser -->|Push購読登録| api
    sw -.通知表示.-> user

    esp -->|センサーデータ送信| api

    api --> chat
    api --> sensor
    ws --> chat

    chat -->|プロフィール取得| db
    chat -->|直近会話取得| db
    chat --> rag
    rag -->|関連記憶検索| vector
    rag -->|埋め込み/応答生成| llm
    chat -->|会話履歴保存| db
    chat -->|会話の長期記憶化| vector
    chat -->|応答ストリーミング| ws

    sensor -->|最新状態更新| db
    sensor -->|イベント検知| push
    sensor -->|イベント通知| ws
    push -->|購読情報取得| db
    push -->|通知送信| pushsvc
    pushsvc -->|Push| sw

    db --> auth
```

## データフロー

### 1. チャット

1. ユーザーが `Next.js` フロントエンドからメッセージを送信
2. `Go/Gin` バックエンドが `Supabase PostgreSQL` からプロフィール・直近履歴・センサー状態を取得
3. `pgvector` から関連する長期記憶を検索し、`RAG` 用コンテキストを構築
4. `Gemini` が返答を生成し、`WebSocket` でフロントへストリーミング送信
5. 会話ログは短期記憶として保存し、1往復ごとにベクトル化して長期記憶へ保存

### 2. センサー連携

1. `ESP32` が土壌水分・照度・温度をバックエンドへ送信
2. `Sensor Service` が最新状態を保存し、水やりや乾燥などのイベントを判定
3. イベント発生時は `WebSocket` でアプリへ即時反映
4. 必要に応じて `Web Push` でアプリ外にも通知

## ポスター向けに強調するとよい点

- `ESP32` による実世界センシング
- `RAG` による長期記憶検索
- `Gemini` を使った文脈依存の対話生成
- `WebSocket` によるリアルタイム会話
- `Web Push` による自発的な通知
