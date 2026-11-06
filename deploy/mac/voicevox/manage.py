#!/usr/bin/env python3
"""Start and verify the dedicated, loopback-only VOICEVOX container."""

import argparse
import array
import io
import json
import math
from pathlib import Path
import shutil
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import wave


HERE = Path(__file__).resolve().parent
ENGINE_URL = "http://127.0.0.1:50021"
VERSION = "0.26.1"
SPEAKER_NAME = "青山龍星"
STYLE_NAME = "ノーマル"
SAMPLE_TEXT = "こんにちは。ことの葉です。今日も、ゆっくりお話ししましょう。"
CREDIT = "VOICEVOX:青山龍星"


def compose(*arguments):
    docker = shutil.which("docker")
    if not docker:
        candidate = Path.home() / ".orbstack" / "bin" / "docker"
        if candidate.is_file():
            docker = str(candidate)
    if not docker:
        raise RuntimeError("Docker CLI is unavailable. Start OrbStack first.")
    subprocess.run([docker, "compose", "-f", str(HERE / "compose.yaml"), *arguments],
                   check=True, timeout=1200 if arguments[0] in ("pull", "up") else 60)


def request(path, body=None, method="GET", timeout=30):
    payload = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(ENGINE_URL + path, data=payload, method=method,
                                 headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=timeout) as response:
        return response.read()


def status():
    version = json.loads(request("/version", timeout=3))
    if version != VERSION:
        raise RuntimeError(f"Unexpected engine version: {version}; expected {VERSION}.")
    speakers = json.loads(request("/speakers", timeout=5))
    for speaker in speakers:
        if speaker["name"] == SPEAKER_NAME:
            for style in speaker["styles"]:
                if style["name"] == STYLE_NAME:
                    return {"ready": True, "version": version, "url": ENGINE_URL,
                            "speaker": SPEAKER_NAME, "style": STYLE_NAME,
                            "speaker_id": style["id"], "credit": CREDIT}
    raise RuntimeError("The configured speaker/style is unavailable.")


def start():
    compose("up", "-d", "engine")
    deadline = time.monotonic() + 120
    while time.monotonic() < deadline:
        try:
            return status()
        except (urllib.error.URLError, TimeoutError, OSError):
            time.sleep(1)
    raise RuntimeError("VOICEVOX did not become ready; inspect its compose logs.")


def sample(output):
    state = status()
    started = time.monotonic()
    params = urllib.parse.urlencode({"text": SAMPLE_TEXT, "speaker": state["speaker_id"]})
    query = json.loads(request("/audio_query?" + params, method="POST"))
    query_seconds = time.monotonic() - started
    query["speedScale"] = 1.12
    synthesis_started = time.monotonic()
    audio = request("/synthesis?speaker=" + str(state["speaker_id"]), body=query, method="POST", timeout=60)
    synthesis_seconds = time.monotonic() - synthesis_started
    with wave.open(io.BytesIO(audio), "rb") as wav:
        channels, width, rate, frames = wav.getnchannels(), wav.getsampwidth(), wav.getframerate(), wav.getnframes()
        if wav.getcomptype() != "NONE" or width != 2 or channels != 1 or rate <= 0 or frames == 0:
            raise RuntimeError("Unexpected WAV format or empty synthesis.")
        samples = array.array("h", wav.readframes(frames))
    if sys.byteorder != "little":
        samples.byteswap()
    if len(samples) != frames:
        raise RuntimeError("The WAV frame data is truncated.")
    peak = max(abs(value) for value in samples)
    rms = math.sqrt(sum(value * value for value in samples) / len(samples))
    if peak == 0 or rms == 0:
        raise RuntimeError("The synthesized WAV is silent.")
    output = output.resolve()
    credit_file = output.with_suffix(".txt")
    if output.exists() or credit_file.exists():
        raise RuntimeError("Sample output already exists; choose a new path.")
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_bytes(audio)
    credit_file.write_text(f"{CREDIT}\nStyle: {STYLE_NAME}\nEngine: {VERSION}\nSpeed: 1.12\nText: {SAMPLE_TEXT}\n"
                           "Terms: https://voicevox.hiroshiba.jp/term/\n"
                           "Voice terms: https://www.virvoxproject.com/voicevoxの利用規約\n", encoding="utf-8")
    return {**state, "output": str(output), "bytes": len(audio), "sample_rate_hz": rate,
            "channels": channels, "sample_width_bits": width * 8,
            "duration_seconds": round(frames / rate, 3), "peak": peak,
            "rms": round(rms, 2), "clipped_samples": sum(abs(value) >= 32767 for value in samples),
            "audio_query_seconds": round(query_seconds, 3), "synthesis_seconds": round(synthesis_seconds, 3)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    for name in ("pull", "start", "stop", "status"):
        commands.add_parser(name)
    sample_parser = commands.add_parser("sample")
    sample_parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        if args.command == "pull":
            compose("pull", "engine")
        elif args.command == "stop":
            compose("stop", "engine")
        elif args.command == "start":
            print(json.dumps(start(), ensure_ascii=False, indent=2))
        elif args.command == "status":
            print(json.dumps(status(), ensure_ascii=False, indent=2))
        else:
            print(json.dumps(sample(args.output), ensure_ascii=False, indent=2))
    except (RuntimeError, urllib.error.URLError, OSError, ValueError, subprocess.SubprocessError, wave.Error) as error:
        print(f"VOICEVOX: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
