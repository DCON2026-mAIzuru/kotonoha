# DL・フロント統合 実装設計・計画書

## 方針

`DL/bonsAI.py` は変更しない。

盆栽画像診断は、`bonsAI.py` を直接フロントや Go バックエンドに組み込むのではなく、FastAPI の DL サービスで包む。フロントは既存どおり Go バックエンドだけを呼び、Go バックエンドが認証・DB保存・FastAPI 呼び出しを担当する。

```text
Frontend
  -> Go Backend
    -> FastAPI DL Service
      -> DL/runner.py
        -> DL/bonsAI.py
    -> Supabase
  -> Frontend
```

この構成にする理由:

- `bonsAI.py` を壊さずに統合できる
- PyTorch / OpenCV / YOLO / MobileSAM の重い依存を Go バックエンドから分離できる
- FastAPI 起動時にモデルを常駐ロードする余地がある
- フロントから DL サービスを直接公開せず、認証とユーザー紐づけを Go に集約できる
- 将来 GPU 環境や別ホストへ DL だけ移しやすい

## ゴール

### MVP

ログイン済みユーザーがフロントから盆栽画像をアップロードし、葉色解析による診断結果を確認できる。

MVP で返す情報:

- 健康度スコア
- 状態判定
- 深刻度
- 詳細説明
- 全体・新芽・古葉の色割合
- 可視化画像の URL
- 診断日時

### 次フェーズ

- 最新画像診断結果をチャットプロンプトへ反映する
- 危険診断を画面内イベントまたは Push 通知に接続する
- 診断履歴を UI で確認できるようにする

## 現状整理

### 既存システム

- フロントエンド: `frontend/` の Next.js App Router
- バックエンド: `backend/` の Go/Gin API
- DB: Supabase PostgreSQL
- 認証: Supabase Auth + Go 側 JWT 検証
- センサー状態: `sensor_data` にユーザーごとの最新値を保持
- チャット: WebSocket で LLM ストリーミング、センサー状態をプロンプトに反映

### DL 側

- 既存実装: `DL/bonsAI.py`
- モデル:
  - `DL/best.pt`
  - `DL/mobile_sam.pt`
- 入力: ローカル画像パス
- 出力:
  - 標準出力に健康診断結果
  - `output_1_yolo_detection.jpg`
  - `output_2_advanced_sam_segmentation.jpg`
  - `output_3_advanced_color_extraction.jpg`
- 制約:
  - JSON を返さない
  - 出力ファイル名が固定
  - 実行時のカレントディレクトリに画像を出力する
  - モデルロードと推論が重い

## 追加する構成

```text
DL/
  bonsAI.py                  # 既存。変更しない
  api.py                     # 新規。FastAPI エントリーポイント
  runner.py                  # 新規。bonsAI.py 呼び出し・stdout パース
  schemas.py                 # 新規。FastAPI のレスポンス型
  requirements.txt           # 新規。DL サービス依存関係
  README.md                  # 任意。DL サービス起動手順
```

Go 側:

```text
backend/internal/api/handlers/diagnosis_handler.go
backend/internal/service/diagnosis_service.go
backend/internal/models/diagnosis.go
backend/internal/repository/diagnosis_repository.go
backend/internal/dlclient/client.go
```

フロント側:

```text
frontend/src/components/diagnosis-panel.tsx
frontend/src/lib/backend-api.ts
frontend/src/types/api.ts
```

## 責務分担

| 層 | 責務 |
| --- | --- |
| Frontend | 画像選択、アップロード、診断結果表示、ローディング・エラー表示 |
| Go Backend | JWT 認証、ファイル検証、FastAPI 呼び出し、DB 保存、フロント向けレスポンス整形 |
| FastAPI DL Service | 画像一時保存、`bonsAI.py` 実行、標準出力パース、可視化画像回収 |
| Supabase | 診断結果、メタデータ、可視化画像パスの保存 |

## FastAPI DL サービス設計

### エンドポイント

外部公開はしない。Go バックエンドからのみ呼ぶ想定。

```text
GET  /healthz
POST /v1/diagnose
```

### POST /v1/diagnose 入力

`multipart/form-data`

```text
image: jpg/png/webp
request_id: string
```

`request_id` は Go バックエンドが発行する UUID。DL サービス内の作業ディレクトリ名に使う。

### POST /v1/diagnose 出力

```json
{
  "request_id": "uuid",
  "health_score": 87.3,
  "status": "healthy",
  "status_label": "健康",
  "severity": "none",
  "severity_label": "なし",
  "detail": "新芽の緑が十分に保たれており、全体として良好です。",
  "ratios": {
    "total": { "green": 95.1, "yellow": 3.2, "brown_red": 1.7 },
    "top": { "green": 96.8, "yellow": 2.1, "brown_red": 1.1 },
    "bottom": { "green": 92.4, "yellow": 5.0, "brown_red": 2.6 }
  },
  "artifacts": {
    "yolo_detection": "/work/uuid/output_1_yolo_detection.jpg",
    "sam_segmentation": "/work/uuid/output_2_advanced_sam_segmentation.jpg",
    "color_extraction": "/work/uuid/output_3_advanced_color_extraction.jpg"
  },
  "raw_stdout": "..."
}
```

Go バックエンドが `artifacts` のファイルを取得または保存する。フロントには DL サービス内部パスを返さない。

### runner.py の役割

`runner.py` は `bonsAI.py` を import して、`analyze_health_advanced()` を呼び出す。

重要な実装ルール:

- `bonsAI.py` は変更しない
- リクエストごとに作業ディレクトリを作る
- `os.chdir(work_dir)` してから `analyze_health_advanced()` を呼ぶ
- `yolo_weights` と `sam_weights` は絶対パスで渡す
- `stdout` をキャプチャし、健康度・割合・状態判定を正規表現で抽出する
- `output_*.jpg` を作業ディレクトリから回収する
- 例外は FastAPI 側で JSON エラーに変換する

同時実行時の注意:

- `bonsAI.py` は固定ファイル名へ出力するため、作業ディレクトリ分離が必須
- `os.chdir()` はプロセス全体へ効くため、初期実装では診断処理を 1 並列に制限する
- 複数同時診断が必要になったら、`ProcessPoolExecutor` またはジョブキューへ移行する

### FastAPI の環境変数

```text
DL_WORK_DIR=/tmp/kotonoha-dl
DL_YOLO_WEIGHTS=/app/DL/best.pt
DL_SAM_WEIGHTS=/app/DL/mobile_sam.pt
DL_MAX_IMAGE_MB=8
DL_ANALYZE_TIMEOUT_SECONDS=60
DL_ALLOWED_ORIGIN=http://localhost:8080
```

`DL_ALLOWED_ORIGIN` は内部ネットワーク運用なら必須ではないが、ローカル検証では Go バックエンドだけを許可する。

## Go バックエンド API 設計

### フロント向け API

認証必須。

```text
POST /api/v1/diagnosis/images
GET  /api/v1/diagnosis/latest
GET  /api/v1/diagnosis/history?limit=20
GET  /api/v1/diagnosis/artifacts/:id/:kind
```

### POST /api/v1/diagnosis/images 入力

`multipart/form-data`

```text
image: jpg/png/webp
```

### POST /api/v1/diagnosis/images 出力

```json
{
  "diagnosis": {
    "id": "uuid",
    "user_id": "uuid",
    "health_score": 87.3,
    "status": "healthy",
    "status_label": "健康",
    "severity": "none",
    "severity_label": "なし",
    "detail": "新芽の緑が十分に保たれており、全体として良好です。",
    "ratios": {
      "total": { "green": 95.1, "yellow": 3.2, "brown_red": 1.7 },
      "top": { "green": 96.8, "yellow": 2.1, "brown_red": 1.1 },
      "bottom": { "green": 92.4, "yellow": 5.0, "brown_red": 2.6 }
    },
    "artifact_urls": {
      "yolo_detection": "/api/v1/diagnosis/artifacts/{id}/yolo_detection",
      "sam_segmentation": "/api/v1/diagnosis/artifacts/{id}/sam_segmentation",
      "color_extraction": "/api/v1/diagnosis/artifacts/{id}/color_extraction"
    },
    "created_at": "2026-04-30T00:00:00Z"
  }
}
```

### Go 側の環境変数

```text
DL_SERVICE_URL=http://127.0.0.1:8090
DL_SERVICE_TIMEOUT_SECONDS=90
DIAGNOSIS_MAX_IMAGE_MB=8
DIAGNOSIS_ARTIFACT_DIR=/tmp/kotonoha-diagnosis-artifacts
```

将来 Supabase Storage を使う場合は、`DIAGNOSIS_ARTIFACT_DIR` の代わりに Storage bucket を使う。

### Go 側の処理フロー

1. JWT からユーザー ID を取得する
2. multipart の画像を読む
3. サイズ、拡張子、MIME type を検証する
4. `request_id` を発行する
5. FastAPI `/v1/diagnose` に画像と `request_id` を送る
6. DL 結果を受け取る
7. 可視化画像を Go 側の artifact 保存先にコピーまたは取得する
8. `bonsai_diagnoses` に保存する
9. フロント向け URL に整形して返す

## DB スキーマ

`sensor_data` は最新センサー値用なので、画像診断は専用テーブルにする。

```sql
create table if not exists public.bonsai_diagnoses (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  health_score numeric(5, 2) not null check (health_score >= 0 and health_score <= 100),
  status text not null,
  status_label text not null,
  severity text not null,
  severity_label text not null,
  detail text not null,
  ratios jsonb not null,
  artifact_paths jsonb,
  raw_result jsonb,
  created_at timestamptz not null default now()
);

create index if not exists idx_bonsai_diagnoses_user_created_at
  on public.bonsai_diagnoses (user_id, created_at desc);

alter table public.bonsai_diagnoses enable row level security;
```

RLS は既存の `sensor_data` や `chat_history` と同じく、自分の `user_id` のみ select/insert/update/delete 可能にする。

## フロントエンド設計

### 追加 UI

`DiagnosisPanel` を追加する。

表示内容:

- 画像アップロードボタン
- 選択画像のプレビュー
- 診断中ローディング
- 健康度スコア
- 状態判定、深刻度、詳細説明
- 葉色割合
- 可視化画像
- 最新診断結果の再取得

### 配置

初期はチャット画面の補助パネルまたはモーダルとして追加する。既存の `プロフィール編集`、`通知設定` と同じように、ヘッダー操作から開ける導線が自然。

スマホでは画面占有が大きいため、チャット本体とは同時表示せず、モーダルまたは下部シートとして扱う。

### 型定義

`frontend/src/types/api.ts` に追加する。

```ts
export interface DiagnosisColorRatios {
  green: number;
  yellow: number;
  brown_red: number;
}

export interface BonsaiDiagnosis {
  id: string;
  user_id: string;
  health_score: number;
  status: string;
  status_label: string;
  severity: string;
  severity_label: string;
  detail: string;
  ratios: {
    total: DiagnosisColorRatios;
    top: DiagnosisColorRatios;
    bottom: DiagnosisColorRatios;
  };
  artifact_urls: Partial<Record<'yolo_detection' | 'sam_segmentation' | 'color_extraction', string>>;
  created_at: string;
}
```

## チャット連携

MVP の後、`ChatService` が最新の `bonsai_diagnoses` を取得して、センサー状態と同じようにシステムプロンプトへ差し込む。

追加する文脈:

```text
画像診断:
- 健康度: 87.3 / 100
- 状態: 健康
- 深刻度: なし
- 葉色: 緑 95.1%, 黄 3.2%, 茶 1.7%
- 診断時刻: 2026-04-30 16:00
```

これにより、「水やりは必要？」「葉の色は大丈夫？」のような質問に、センサー値と画像診断の両方を踏まえて返答できる。

## イベント・Push 連携

初期は Push しない。画像診断はユーザー操作で発火するため、まずは画面内で結果を見せる。

後続でイベント化する候補:

- `image_diagnosis_warning`
- `image_diagnosis_critical`
- `leaf_yellowing_detected`
- `leaf_browning_detected`

初期発火条件:

- `severity = high`
- `health_score < 50`
- `total.brown_red > 35`
- `top.brown_red > 10`
- `top.yellow > 7`

Push に進める場合は、既存のセンサーイベントと同じくクールダウンを設ける。

## 実装順

1. `DL/runner.py` を追加し、`bonsAI.py` を変更せずに固定画像で診断できるようにする
2. `DL/api.py` を追加し、`POST /v1/diagnose` で画像診断できるようにする
3. FastAPI のローカル起動手順と `DL/requirements.txt` を追加する
4. `bonsai_diagnoses` migration を追加する
5. Go バックエンドに `dlclient` と診断 API を追加する
6. Go バックエンドで診断結果を DB 保存し、artifact URL を返す
7. フロントに `DiagnosisPanel` と API 関数を追加する
8. 最新診断結果をチャットプロンプトへ反映する
9. 危険診断の画面内イベント化を追加する
10. 必要になったら Push 連携を追加する

## MVP 受け入れ条件

- `bonsAI.py` を変更していない
- FastAPI の `/healthz` が通る
- FastAPI の `/v1/diagnose` が jpg/png/webp を受け取り JSON を返す
- Go の `POST /api/v1/diagnosis/images` はログイン済みユーザーだけが呼べる
- 診断結果が `bonsai_diagnoses` に保存される
- フロントで画像アップロード、診断中表示、結果表示ができる
- 可視化画像をフロントで確認できる
- 葉が検出できない、画像が不正、DL が失敗した場合に UI が壊れない
- `go test ./...` と `npm run build` が通る

## リスクと対策

| リスク | 対策 |
| --- | --- |
| `bonsAI.py` が固定ファイル名に出力する | リクエストごとに作業ディレクトリを分ける |
| `os.chdir()` が同時実行に弱い | MVP は 1 並列に制限し、必要ならプロセス分離へ移行 |
| 標準出力パースが壊れやすい | `runner.py` にパーステストを追加する |
| モデルロードが重い | FastAPI 常駐化し、次段階でモデルロードを runner 側にキャッシュする |
| DL 依存でデプロイが重い | Go と別コンテナ・別ホストにする |
| 画像サイズが大きすぎる | Go と FastAPI の両方でサイズ上限を設ける |
| 可視化画像の権限管理 | フロントには Go の認証付き artifact API だけを返す |

## ローカル起動イメージ

FastAPI:

```bash
cd DL
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
uvicorn api:app --host 127.0.0.1 --port 8090
```

Go:

```bash
cd backend
DL_SERVICE_URL=http://127.0.0.1:8090 go run ./cmd/server
```

Frontend:

```bash
cd frontend
npm run dev
```

## デプロイ方針

MVP はローカル検証を優先する。

本番または展示環境では、Go バックエンドと FastAPI DL サービスを別プロセスまたは別コンテナに分ける。フロントから見える公開 API は Go バックエンドだけにする。

```text
Vercel Frontend
  -> Fly.io Go Backend
    -> DL Service
    -> Supabase
```

DL Service の配置候補:

- Fly.io の別 app
- GPU が必要なら GPU 対応ホスト
- 学内・展示用 LAN 内の専用マシン

## 未決事項

| 項目 | 初期方針 |
| --- | --- |
| artifact 保存先 | MVP は Go 側ローカル保存。履歴保持が必要なら Supabase Storage |
| FastAPI の公開範囲 | 外部公開しない。Go バックエンドからのみ呼ぶ |
| 同時実行数 | MVP は 1。必要になったらプロセス分離 |
| 診断履歴 UI | 初期は最新のみ。DB は履歴保存 |
| Push 連携 | 初期は実装しない。画面内イベント後に検討 |
