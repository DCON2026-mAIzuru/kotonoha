# Mac local semantic embeddings

This service embeds Japanese and other supported languages using the official MIT-licensed `intfloat/multilingual-e5-small` model. It listens only on `127.0.0.1:50022`. The Go backend calls it locally; do not add a public proxy route or call it directly from the browser. No paid API, Colab process, GPU, or separate Docker VM is used.

The official ONNX int8 artifact is pinned to revision `614241f622f53c4eeff9890bdc4f31cfecc418b3`; model and tokenizer SHA-256 values are checked on download and at every startup. Its `avx512_vnni` filename describes the exported quantization configuration: this service executes the graph with the native ARM64 ONNX Runtime CPU provider, without x86 emulation. Python is pinned to 3.12.12. `requirements.txt` pins every package and wheel hash; `requirements.in` records the direct dependencies.

## Run

From the repository root:

```sh
python3 deploy/mac/embeddings/manage.py install
python3 deploy/mac/embeddings/manage.py start
python3 deploy/mac/embeddings/manage.py status
python3 deploy/mac/embeddings/manage.py verify
python3 deploy/mac/embeddings/manage.py restart
python3 deploy/mac/embeddings/manage.py stop
```

`install` needs internet access and `uv` on PATH or at `~/.local/bin/uv`. All Python installations, dependency/model caches, PID state and logs stay in the ignored `.runtime/embeddings/` directory. `start` requires the downloaded files and works offline. It warms up the model before reporting ready, starts a detached process, and refuses duplicate or unrelated listeners. `stop` sends SIGTERM only to the recorded embedding process, drains active requests, and never force-kills it. `restart` performs that graceful stop and then starts the pinned model. After a Mac reboot, run `start` again; this module does not install a login item or change other services.

There are no secrets or required environment variables for this service. Configure the backend with:

```dotenv
EMBEDDING_PROVIDER=local
EMBEDDING_BASE_URL=http://127.0.0.1:50022
EMBEDDING_TIMEOUT_SECONDS=30
```

Start this service before the backend. The backend reads `/metadata` at startup, verifies the canonical configuration hash, caches its embedding space, and checks that every `/embed` response uses the same space. No separate environment variable supplies a space ID.

## API and storage space

`GET /health` returns `{status:"ok", service, embedding_space, model_id, dimension, uptime_seconds, inference_busy}` only after the model is ready. `GET /metadata` returns the full immutable configuration from `contract.py`, including model ID/revision, dimension, normalization, prefixes, tokenizer/model hashes, runtime versions, and the embedding-space ID.

```sh
curl --fail http://127.0.0.1:50022/metadata
curl --fail http://127.0.0.1:50022/embed \
  -H 'Content-Type: application/json' \
  -d '{"text":"葉っぱが黄色く変わってきた","kind":"query"}'
```

`POST /embed` requires exactly `text` (a nonblank string of at most 16,000 characters) and `kind` (`query` or `passage`). Callers send raw text: the service adds `query: ` or `passage: `, including for Japanese. The JSON response has `embedding` (384 finite floats with unit L2 norm), `dimension`, `embedding_space`, `token_count` and `truncated`. Input is truncated at 512 tokens including the prefix/special tokens. Pooling averages the attention-masked final hidden states, then normalizes in float32. The service never returns hash-derived or placeholder vectors.

The Go backend adds zeros on the right to store 384 values in its existing 3072-dimensional vector column. Right zero padding preserves dot products and L2 norm. The stable space ID is a SHA-256 of the complete model, quantization, pooling, prefix, normalization, runtime and padding configuration. A change requires a new space and re-embedding; vectors from different spaces must never be searched together. No padding occurs in this HTTP service.

The CPU session uses two intra-op threads, one inter-op thread, sequential execution and no idle spinning. Only one inference runs at a time; overlap returns HTTP 429 (`inference_busy`) instead of an unbounded queue. At most four HTTP request handlers and eight pending connections are allowed. Bodies are limited to 64 KiB, socket reads time out in ten seconds, and unexpected Host/Origin values are rejected. No CORS is enabled. Inputs, vectors, headers and arbitrary URLs are not logged. A process running as another local user could still call the loopback API; this is a local deployment boundary, not an authenticated public service.

`verify` runs the actual model and checks 384-dimensional finite unit vectors, two Japanese paraphrase rankings against topical and unrelated distractors, 512-token truncation, overlapping inference rejection, and request bounds. It prints timings and similarity scores without printing vectors or private application text. These small functional examples confirm operation; they are not a general retrieval-quality benchmark.

On the deployment Mac on 2026-11-06, both Japanese retrieval examples ranked the expected passage first (cosine scores 0.887 and 0.847). Five short requests had a median latency of 5.3 ms and maximum of 6.5 ms. The warm process used about 712 MiB RSS; context length, concurrent Mac workloads and operating-system memory accounting can change these figures. The installed runtime and caches used about 442 MiB on disk. Graceful stop/restart and the `127.0.0.1` listener were also verified.

## Official references

- [Model card and MIT license metadata](https://huggingface.co/intfloat/multilingual-e5-small)
- [Pinned official ONNX files](https://huggingface.co/intfloat/multilingual-e5-small/tree/614241f622f53c4eeff9890bdc4f31cfecc418b3/onnx)
- [ONNX Runtime threading controls](https://onnxruntime.ai/docs/performance/tune-performance/threading.html)
