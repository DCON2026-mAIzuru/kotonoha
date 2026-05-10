# Kotonoha DL Service

`bonsAI.py` は変更せず、FastAPI からラップして画像診断を提供します。

## ローカル起動

```bash
cd DL
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
uvicorn api:app --host 127.0.0.1 --port 8090
```

確認:

```bash
curl -s http://127.0.0.1:8090/healthz
```

Go バックエンド側は `DL_SERVICE_URL=http://127.0.0.1:8090` を設定して起動します。
