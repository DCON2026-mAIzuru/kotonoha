#!/usr/bin/env python3
"""Loopback-only E5 inference. No request texts or vectors are logged."""

import argparse
import hashlib
from importlib.metadata import version
import json
import logging
import os
from pathlib import Path
import signal
import socket
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

os.environ["TOKENIZERS_PARALLELISM"] = "false"
os.environ["OMP_NUM_THREADS"] = "2"
os.environ["OPENBLAS_NUM_THREADS"] = "2"

import numpy as np
import onnxruntime as ort
from tokenizers import Tokenizer

from contract import EMBEDDING_SPACE, FILES, HOST, METADATA, PORT, SERVICE

MAX_BODY_BYTES = 65536
MAX_TEXT_CHARACTERS = 16000
LOG = logging.getLogger(SERVICE)


def verify_files(model_dir):
    for filename, expected_hash in FILES.items():
        path = model_dir / Path(filename).name
        digest = hashlib.sha256()
        with path.open("rb") as stream:
            for block in iter(lambda: stream.read(1024 * 1024), b""):
                digest.update(block)
        if digest.hexdigest() != expected_hash:
            raise RuntimeError("Model file checksum mismatch; run manage.py install")


class Embedder:
    def __init__(self, model_dir):
        for package, expected in (("onnxruntime", "1.31.1"), ("tokenizers", "0.23.3"), ("numpy", "2.5.4")):
            if version(package) != expected:
                raise RuntimeError("Dependency version mismatch; run manage.py install")
        verify_files(model_dir)
        self.tokenizer = Tokenizer.from_file(str(model_dir / "tokenizer.json"))
        self.tokenizer.enable_truncation(max_length=512)
        options = ort.SessionOptions()
        options.intra_op_num_threads = 2
        options.inter_op_num_threads = 1
        options.execution_mode = ort.ExecutionMode.ORT_SEQUENTIAL
        options.graph_optimization_level = ort.GraphOptimizationLevel.ORT_ENABLE_ALL
        options.add_session_config_entry("session.intra_op.allow_spinning", "0")
        options.add_session_config_entry("session.inter_op.allow_spinning", "0")
        self.session = ort.InferenceSession(
            str(model_dir / "model_qint8_avx512_vnni.onnx"),
            sess_options=options,
            providers=["CPUExecutionProvider"],
        )
        self.lock = threading.Lock()
        self.embed("接続確認", "query")

    def embed(self, text, kind):
        encoded = self.tokenizer.encode(METADATA[kind + "_prefix"] + text)
        arrays = {
            "input_ids": np.array([encoded.ids], dtype=np.int64),
            "attention_mask": np.array([encoded.attention_mask], dtype=np.int64),
            "token_type_ids": np.array([encoded.type_ids], dtype=np.int64),
        }
        inputs = {item.name: arrays[item.name] for item in self.session.get_inputs()}
        hidden = self.session.run(None, inputs)[0]
        if hidden.shape != (1, len(encoded.ids), 384):
            raise RuntimeError("Unexpected model output shape")
        mask = arrays["attention_mask"][..., None].astype(np.float32)
        pooled = (hidden.astype(np.float32) * mask).sum(axis=1) / mask.sum(axis=1)
        norm = np.linalg.norm(pooled, axis=1, keepdims=True)
        if not np.all(np.isfinite(pooled)) or np.any(norm <= 0):
            raise RuntimeError("Invalid model output")
        vector = (pooled / norm)[0]
        return {
            "embedding": vector.tolist(),
            "embedding_space": EMBEDDING_SPACE,
            "dimension": 384,
            "token_count": len(encoded.ids),
            "truncated": bool(encoded.overflowing),
        }


class BoundedServer(ThreadingHTTPServer):
    daemon_threads = False
    block_on_close = True
    request_queue_size = 8

    def __init__(self, engine):
        self.engine = engine
        self.started_at = time.monotonic()
        self.request_slots = threading.BoundedSemaphore(4)
        super().__init__((HOST, PORT), Handler)

    def get_request(self):
        connection, address = super().get_request()
        connection.settimeout(10)
        return connection, address

    def process_request(self, request, client_address):
        if not self.request_slots.acquire(blocking=False):
            try:
                request.sendall(b"HTTP/1.0 503 Service Unavailable\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
            except OSError:
                pass
            self.shutdown_request(request)
            return
        try:
            super().process_request(request, client_address)
        except BaseException:
            self.request_slots.release()
            raise

    def process_request_thread(self, request, client_address):
        try:
            super().process_request_thread(request, client_address)
        finally:
            self.request_slots.release()

    def handle_error(self, request, client_address):
        LOG.warning("Request handler failed; no request data recorded")


class Handler(BaseHTTPRequestHandler):
    server_version = SERVICE
    sys_version = ""

    def log_message(self, format, *args):
        # Do not log URL, request bodies, vectors, or arbitrary header content.
        pass

    def send_json(self, status, value):
        payload = json.dumps(value, ensure_ascii=False, allow_nan=False).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(payload)))
        self.send_header("Cache-Control", "no-store")
        self.send_header("X-Content-Type-Options", "nosniff")
        self.send_header("Connection", "close")
        self.end_headers()
        self.close_connection = True
        try:
            self.wfile.write(payload)
        except (BrokenPipeError, ConnectionResetError):
            pass

    def check_local_request(self):
        if self.headers.get("Host") not in ("127.0.0.1:50022", "localhost:50022"):
            self.send_json(403, {"error": "invalid_host"})
            return False
        # This service is for the Mac backend, never for direct browser requests.
        if self.headers.get("Origin"):
            self.send_json(403, {"error": "browser_origin_not_allowed"})
            return False
        return True

    def do_GET(self):
        if not self.check_local_request():
            return
        if self.path == "/metadata":
            self.send_json(200, METADATA)
        elif self.path == "/health":
            self.send_json(200, {
                "status": "ok", "service": SERVICE, "embedding_space": EMBEDDING_SPACE,
                "model_id": METADATA["model_id"], "dimension": 384,
                "uptime_seconds": round(time.monotonic() - self.server.started_at, 1),
                "inference_busy": self.server.engine.lock.locked(),
            })
        else:
            self.send_json(404, {"error": "not_found"})

    def do_POST(self):
        if not self.check_local_request():
            return
        if self.path != "/embed":
            self.send_json(404, {"error": "not_found"})
            return
        if self.headers.get("Content-Type", "").split(";")[0].strip().lower() != "application/json":
            self.send_json(415, {"error": "json_required"})
            return
        lengths = self.headers.get_all("Content-Length", [])
        if self.headers.get("Transfer-Encoding") or len(lengths) != 1:
            self.send_json(400, {"error": "content_length_required"})
            return
        try:
            length = int(lengths[0])
        except ValueError:
            length = -1
        if not 0 < length <= MAX_BODY_BYTES:
            self.send_json(413, {"error": "request_too_large_or_empty"})
            return
        try:
            body = self.rfile.read(length)
            if len(body) != length:
                raise ValueError()
            value = json.loads(body)
        except (ValueError, UnicodeError, RecursionError):
            self.send_json(400, {"error": "invalid_json"})
            return
        except (TimeoutError, socket.timeout):
            self.send_json(408, {"error": "request_timeout"})
            return
        if not isinstance(value, dict) or set(value) != {"text", "kind"}:
            self.send_json(400, {"error": "expected_text_and_kind"})
            return
        text, kind = value["text"], value["kind"]
        if not isinstance(text, str) or not text.strip() or len(text) > MAX_TEXT_CHARACTERS:
            self.send_json(400, {"error": "text_must_have_1_to_16000_characters"})
            return
        if kind not in ("query", "passage"):
            self.send_json(400, {"error": "kind_must_be_query_or_passage"})
            return
        if not self.server.engine.lock.acquire(blocking=False):
            self.send_json(429, {"error": "inference_busy"})
            return
        try:
            result = self.server.engine.embed(text, kind)
        except Exception as error:
            LOG.error("Inference failed (%s); no input recorded", type(error).__name__)
            self.send_json(500, {"error": "inference_failed"})
        else:
            self.send_json(200, result)
        finally:
            self.server.engine.lock.release()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--runtime-dir", type=Path, required=True)
    args = parser.parse_args()
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    engine = Embedder(args.runtime_dir / "model")
    server = BoundedServer(engine)
    stopping = threading.Event()

    def stop(signum, frame):
        if not stopping.is_set():
            stopping.set()
            threading.Thread(target=server.shutdown, daemon=True).start()

    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    LOG.info("Ready at http://%s:%s; CPU threads=2; space=%s", HOST, PORT, EMBEDDING_SPACE)
    try:
        server.serve_forever(poll_interval=0.2)
    finally:
        server.server_close()
        LOG.info("Stopped after draining active requests")


if __name__ == "__main__":
    main()
