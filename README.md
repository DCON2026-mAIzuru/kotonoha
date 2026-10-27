# Kotonoha

盆栽と会話しながら育てるプロジェクトです。センサーの値や盆栽のプロフィール、会話履歴をもとに返答を生成し、画像による健康診断や通知も提供します。

## 主な機能

- Supabase Auth によるメール・パスワード認証、Google ログイン
- 盆栽の名前、性格、背景、話し方の設定となじみ具合の管理
- WebSocket によるストリーミングチャット、会話履歴とベクトル記憶を使った応答
- センサーデータの取得・送信、水分制御、乾燥や水やりのイベント検知
- 画像診断、診断履歴、診断画像の表示
- VOICEVOX による読み上げ、Web Push 通知、PWA 対応

## ドキュメント

- [ドキュメント目次](docs/index.md)
- [ローカル開発・運用](docs/setup/local-development.md)
- [システム構成](docs/Architecture/kotonoha-system.md)
- [API と WebSocket](docs/api.md)
