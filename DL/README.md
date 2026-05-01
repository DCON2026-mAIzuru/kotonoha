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

`MobileSAM` のインストールは GitHub から行います。`pip install -r requirements.txt` が次の行で止まって見える場合があります。

```text
Running command git clone --filter=blob:none --quiet https://github.com/ChaoningZhang/MobileSAM.git ...
```

この表示は `pip` が quiet mode で clone しているため、進捗が出ないだけの場合があります。2〜3分待っても進まない場合は、一度中断して次を実行してください。

```bash
pip install --upgrade pip setuptools wheel
pip install -r requirements.txt -v
```

それでも GitHub clone で止まる場合は、MobileSAM だけ手動で入れます。

```bash
cd /tmp
git clone --depth 1 https://github.com/ChaoningZhang/MobileSAM.git
cd MobileSAM
pip install .
cd /Users/apple/Documents/teamproject/kotonoha/DL
pip install -r requirements.txt
```

手動 clone も止まる場合は、Python ではなく GitHub へのネットワーク接続が原因です。その場合は Wi-Fi を変えるか、次の zip 形式で試してください。

```bash
pip install "https://github.com/ChaoningZhang/MobileSAM/archive/b01a9ccef3b9e10b099b544efe004d0871802c3b.zip"
pip install -r requirements.txt
```

確認:

```bash
curl -s http://127.0.0.1:8090/healthz
```

Go バックエンド側は `DL_SERVICE_URL=http://127.0.0.1:8090` を設定して起動します。
