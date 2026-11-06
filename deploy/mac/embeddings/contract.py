"""Pinned embedding geometry; changes intentionally produce a new storage space."""

import hashlib
import json

SERVICE = "kotonoha-local-embeddings"
HOST = "127.0.0.1"
PORT = 50022
MODEL_ID = "intfloat/multilingual-e5-small"
REVISION = "614241f622f53c4eeff9890bdc4f31cfecc418b3"
FILES = {
    "onnx/model_qint8_avx512_vnni.onnx": "dd476dd0c2514e9b9be83aeb3853fac0763e0bdf4a71645407587d77c48a2d88",
    "onnx/tokenizer.json": "0b44a9d7b51c3c62626640cda0e2c2f70fdacdc25bbbd68038369d14ebdf4c39",
}
SPACE_CONFIG = {
    "model_id": MODEL_ID,
    "revision": REVISION,
    "dimension": 384,
    "normalized": True,
    "query_prefix": "query: ",
    "passage_prefix": "passage: ",
    "max_tokens": 512,
    "pooling": "attention-mask-mean",
    "normalization": "l2-float32",
    "onnx_file": "onnx/model_qint8_avx512_vnni.onnx",
    "onnx_sha256": FILES["onnx/model_qint8_avx512_vnni.onnx"],
    "tokenizer_sha256": FILES["onnx/tokenizer.json"],
    "runtime": "onnxruntime-1.31.1-cpu",
    "tokenizer_runtime": "tokenizers-0.23.3",
    "padding_dimension": 3072,
    "padding": "right-zero",
}
EMBEDDING_SPACE = "e5-small-v1-" + hashlib.sha256(
    json.dumps(SPACE_CONFIG, sort_keys=True, separators=(",", ":")).encode()
).hexdigest()
METADATA = {"service": SERVICE, "embedding_space": EMBEDDING_SPACE, **SPACE_CONFIG}
