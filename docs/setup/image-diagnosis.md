# 画像診断サービス

`DL/` は `bonsAI.py` の診断処理を FastAPI から呼び出します。バックエンドを通した利用には [ローカル開発・運用](local-development.md) の設定も必要です。

## 起動

```bash
cd DL
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
uvicorn api:app --host 127.0.0.1 --port 8090
```

`GET http://127.0.0.1:8090/healthz` でプロセスの稼働を確認できます。これはモデルの推論成功を保証しません。バックエンドから到達できるよう `DL_SERVICE_URL` を設定してください。

## 設定とデータ

| 環境変数 | 既定値 | 用途 |
| --- | --- | --- |
| `DL_WORK_DIR` | `/tmp/kotonoha-dl` | 診断入力と成果物の保存先 |
| `DL_YOLO_WEIGHTS` | `DL/best.pt` の絶対パス | YOLO 重み |
| `DL_MAX_IMAGE_MB` | `8` | DL サービス側の画像サイズ上限 |
| `DL_CACHE_DIR` | `/tmp/kotonoha-dl-cache` | 推論時のキャッシュ |

バックエンドにも `DIAGNOSIS_MAX_IMAGE_MB`、`DIAGNOSIS_ARTIFACT_DIR`、`DL_SERVICE_TIMEOUT_SECONDS` があります。両サービスのサイズ上限を揃え、成果物が必要な期間保持される保存先を用意してください。DL サービスは JPEG、PNG、WebP を受け付け、`POST /v1/diagnose` で `request_id` と画像を multipart 形式で受信します。ヒートマップは `GET /v1/artifacts/{request_id}/heatmap` で取得します。これらは内部サービス向けで、認証は実装されていません。公開範囲は接続元を考慮して設定してください。
