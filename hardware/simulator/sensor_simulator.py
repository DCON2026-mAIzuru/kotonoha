#!/usr/bin/env python3
"""
盆栽チャットシステム - ESP32 センサーシミュレーター

ESP32 がない状態でデモを行うためのスクリプトです。
バックエンドの POST /api/v1/sensor を直接叩き、
センサーデータの送信・水やりイベントの発火を再現します。

使い方:
  python sensor_simulator.py              # 通常モード (10 分ごとに送信)
  python sensor_simulator.py --demo       # デモモード (水やりイベントを即時発火)
  python sensor_simulator.py --once       # 1 回だけ送信して終了
  python sensor_simulator.py --help       # ヘルプ表示

設定:
  スクリプト下部の "===== 設定 =====" セクションを編集するか、
  環境変数 BACKEND_URL / SENSOR_TOKEN / SENSOR_USER_ID で上書きできます。
"""

import argparse
import json
import os
import random
import sys
import time
from datetime import datetime, timezone, timedelta

try:
    import urllib.request as urlrequest
    import urllib.error as urlerror
except ImportError:
    print("[ERROR] Python 3 が必要です。")
    sys.exit(1)

# =====================================================================
# ===== 設定 =====
# =====================================================================

BACKEND_URL    = os.environ.get("BACKEND_URL",    "https://YOUR_BACKEND_URL/api/v1/sensor")
SENSOR_TOKEN   = os.environ.get("SENSOR_TOKEN",   "YOUR_SECRET_TOKEN")
SENSOR_USER_ID = os.environ.get("SENSOR_USER_ID", "YOUR_SUPABASE_USER_ID")
DEVICE_ID      = os.environ.get("DEVICE_ID",      "simulator-001")

# 通常モードの送信間隔（秒）
INTERVAL_SEC   = int(os.environ.get("INTERVAL_SEC", "600"))  # デフォルト 10 分

# =====================================================================
# ===== センサー値の生成 =====
# =====================================================================

JST = timezone(timedelta(hours=9))

def now_iso() -> str:
    """現在時刻を ISO 8601 (JST) 形式で返す。"""
    return datetime.now(JST).strftime("%Y-%m-%dT%H:%M:%S+09:00")

def make_payload(moisture: float, temperature: float, light_lux: float = 0.0) -> dict:
    return {
        "device_id":           DEVICE_ID,
        "recorded_at":         now_iso(),
        "moisture_percent":    round(moisture, 1),
        "temperature_celsius": round(temperature, 1),
        "light_lux":           round(light_lux, 1),
    }

def random_normal() -> dict:
    """通常運用を模した自然なランダム値を生成する。"""
    moisture    = random.uniform(30.0, 70.0)
    temperature = random.uniform(18.0, 30.0)
    light_lux   = random.uniform(500.0, 15000.0)
    return make_payload(moisture, temperature, light_lux)

# =====================================================================
# ===== HTTP 送信 =====
# =====================================================================

def send(payload: dict) -> tuple[int, dict]:
    """
    バックエンドへ POST する。
    戻り値: (HTTPステータスコード, レスポンスJSON)
    """
    body = json.dumps(payload).encode("utf-8")
    req  = urlrequest.Request(
        BACKEND_URL,
        data=body,
        method="POST",
        headers={
            "Content-Type":      "application/json",
            "X-Sensor-Token":    SENSOR_TOKEN,
            "X-Sensor-User-ID":  SENSOR_USER_ID,
        },
    )
    try:
        with urlrequest.urlopen(req, timeout=15) as resp:
            status = resp.getcode()
            body_bytes = resp.read()
            return status, json.loads(body_bytes) if body_bytes else {}
    except urlerror.HTTPError as e:
        body_bytes = e.read()
        try:
            return e.code, json.loads(body_bytes)
        except Exception:
            return e.code, {"raw": body_bytes.decode("utf-8", errors="replace")}
    except Exception as e:
        return -1, {"error": str(e)}

def print_result(payload: dict, status: int, result: dict) -> None:
    print(f"\n{'─'*60}")
    print(f"  送信時刻  : {payload['recorded_at']}")
    print(f"  水分量    : {payload['moisture_percent']} %")
    print(f"  気温      : {payload['temperature_celsius']} ℃")
    print(f"  照度      : {payload['light_lux']} lux")
    print(f"  HTTP      : {status}")
    if result.get("event_triggered"):
        print(f"  ★ 水やりイベント発火!")
        print(f"  盆栽の反応: {result.get('event_message', '')}")
        print(f"  Push 通知 : {result.get('push_sent_count', 0)} 件送信")
    if result.get("notification_warning"):
        print(f"  [WARN] {result['notification_warning']}")
    print(f"{'─'*60}")

# =====================================================================
# ===== 動作モード =====
# =====================================================================

def mode_once():
    """センサー値を 1 回だけ送信する。"""
    print("[MODE] 1 回送信モード")
    payload = random_normal()
    print(f"[INFO] 送信中: {json.dumps(payload, ensure_ascii=False)}")
    status, result = send(payload)
    print_result(payload, status, result)

def mode_normal():
    """通常モード: INTERVAL_SEC ごとに送信し続ける。Ctrl+C で終了。"""
    print(f"[MODE] 通常モード  (間隔: {INTERVAL_SEC} 秒 / Ctrl+C で終了)")
    while True:
        payload = random_normal()
        print(f"\n[INFO] 送信中: {json.dumps(payload, ensure_ascii=False)}")
        status, result = send(payload)
        print_result(payload, status, result)
        print(f"[INFO] 次の送信まで {INTERVAL_SEC} 秒待機...")
        time.sleep(INTERVAL_SEC)

def mode_demo():
    """
    デモモード: 水やりイベントを即時再現する。

    手順:
      1. 乾燥状態（水分 ~25%）を送信
      2. 30 秒待機（デモ中はその場で「水をあげる演技」をする）
      3. 水やり後（水分 ~70%）を送信
         → バックエンドで +45% 上昇を検知 → LLM が反応 → Push 通知

    バックエンドの発火条件:
      - 3 分以内に moisture_percent が 20% 以上上昇
      - 直近 2 時間に event ロールの履歴がない（クールダウン）
    """
    print("=" * 60)
    print("  デモモード: 水やりイベントを再現します")
    print("=" * 60)

    # --- Step 1: 乾燥状態 ---
    dry_moisture    = random.uniform(20.0, 30.0)
    dry_temperature = random.uniform(20.0, 25.0)
    dry_light       = random.uniform(1000.0, 5000.0)

    payload_dry = make_payload(dry_moisture, dry_temperature, dry_light)
    print(f"\n[1/3] 乾燥状態を送信します (水分 {payload_dry['moisture_percent']}%)")
    status, result = send(payload_dry)
    print_result(payload_dry, status, result)

    if status not in (200, 201):
        print(f"\n[ERROR] 送信失敗 (status={status})。設定を確認してください。")
        print("  BACKEND_URL    :", BACKEND_URL)
        print("  SENSOR_TOKEN   :", SENSOR_TOKEN[:4] + "****" if len(SENSOR_TOKEN) > 4 else "未設定")
        print("  SENSOR_USER_ID :", SENSOR_USER_ID)
        sys.exit(1)

    # --- Step 2: 待機 ---
    wait_sec = 30
    print(f"\n[2/3] {wait_sec} 秒後に水やり後の値を送信します。")
    print("      ← ここでデモ観客に向けて「水をあげる演技」をしてください →")
    for remaining in range(wait_sec, 0, -5):
        print(f"      あと {remaining} 秒...", end="\r", flush=True)
        time.sleep(5)
    print()

    # --- Step 3: 水やり後 ---
    wet_moisture    = dry_moisture + random.uniform(40.0, 50.0)
    wet_moisture    = min(wet_moisture, 98.0)
    wet_temperature = dry_temperature + random.uniform(-1.0, 1.0)
    wet_light       = dry_light

    payload_wet = make_payload(wet_moisture, wet_temperature, wet_light)
    print(f"[3/3] 水やり後の状態を送信します (水分 {payload_wet['moisture_percent']}%)")
    print(f"      差分: +{round(payload_wet['moisture_percent'] - payload_dry['moisture_percent'], 1)}%")
    status, result = send(payload_wet)
    print_result(payload_wet, status, result)

    print("\n[DONE] デモ完了。フロントエンドのチャット画面で盆栽の反応を確認してください。")
    if result.get("push_sent_count", 0) > 0:
        print(f"       Push 通知が {result['push_sent_count']} 件送信されました。スマホでも確認できます。")

# =====================================================================
# ===== エントリポイント =====
# =====================================================================

def validate_config():
    errors = []
    if "YOUR_BACKEND_URL" in BACKEND_URL:
        errors.append("BACKEND_URL が未設定です。スクリプト内または環境変数で設定してください。")
    if "YOUR_SECRET_TOKEN" in SENSOR_TOKEN:
        errors.append("SENSOR_TOKEN が未設定です。")
    if "YOUR_SUPABASE_USER_ID" in SENSOR_USER_ID:
        errors.append("SENSOR_USER_ID が未設定です。")
    if errors:
        print("[ERROR] 設定が不完全です:")
        for e in errors:
            print(f"  - {e}")
        print("\n設定方法:")
        print("  1. スクリプト内の '===== 設定 =====' セクションを直接編集する")
        print("  2. または環境変数で渡す:")
        print("       BACKEND_URL=https://... SENSOR_TOKEN=xxx SENSOR_USER_ID=yyy python sensor_simulator.py --demo")
        sys.exit(1)

def main():
    parser = argparse.ArgumentParser(
        description="盆栽チャットシステム ESP32 センサーシミュレーター",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
使用例:
  python sensor_simulator.py --demo
      水やりイベントを即時再現（デモ推奨）

  python sensor_simulator.py --once
      センサー値を 1 回だけ送信して終了

  python sensor_simulator.py
      通常モード（10 分ごとに送信、Ctrl+C で終了）

  BACKEND_URL=https://app.fly.dev/api/v1/sensor \\
  SENSOR_TOKEN=mysecret \\
  SENSOR_USER_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx \\
  python sensor_simulator.py --demo
        """,
    )
    parser.add_argument(
        "--demo",
        action="store_true",
        help="デモモード: 乾燥→水やり後の値を 30 秒差で送信し、水やりイベントを発火させる",
    )
    parser.add_argument(
        "--once",
        action="store_true",
        help="1 回だけ送信して終了",
    )
    args = parser.parse_args()

    print("盆栽チャットシステム - センサーシミュレーター")
    print(f"  送信先  : {BACKEND_URL}")
    print(f"  User ID : {SENSOR_USER_ID}")
    print(f"  Device  : {DEVICE_ID}")

    validate_config()

    try:
        if args.demo:
            mode_demo()
        elif args.once:
            mode_once()
        else:
            mode_normal()
    except KeyboardInterrupt:
        print("\n\n[INFO] 停止しました。")

if __name__ == "__main__":
    main()
