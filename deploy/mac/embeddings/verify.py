#!/usr/bin/env python3
"""Exercise the real local model, including Japanese paraphrase retrieval."""

from concurrent.futures import ThreadPoolExecutor
import json
import math
import statistics
import time
import urllib.error
import urllib.request

from contract import EMBEDDING_SPACE, METADATA

BASE_URL = "http://127.0.0.1:50022"


def request(path, body=None, headers=None):
    payload = json.dumps(body, ensure_ascii=False).encode() if body is not None else None
    combined = {"Content-Type": "application/json"}
    combined.update(headers or {})
    req = urllib.request.Request(BASE_URL + path, data=payload, headers=combined)
    try:
        with urllib.request.urlopen(req, timeout=30) as response:
            return response.status, json.load(response)
    except urllib.error.HTTPError as error:
        return error.code, json.load(error)


def validate(result):
    assert result["embedding_space"] == EMBEDDING_SPACE
    vector = result["embedding"]
    assert len(vector) == 384 and all(isinstance(x, (float, int)) and math.isfinite(x) for x in vector)
    norm = math.sqrt(sum(x * x for x in vector))
    assert abs(norm - 1.0) < 1e-5, norm
    assert 1 <= result["token_count"] <= 512
    return vector


def main():
    code, metadata = request("/metadata")
    assert code == 200 and metadata == METADATA
    code, health = request("/health")
    assert code == 200 and health["status"] == "ok"
    timings = []

    def embed(text, kind):
        started = time.perf_counter()
        code, value = request("/embed", {"text": text, "kind": kind})
        timings.append((time.perf_counter() - started) * 1000)
        assert code == 200, (code, value.get("error"))
        return validate(value)

    passages = [
        "土が乾燥したら、鉢底から流れ出るまでたっぷり給水してください。",
        "葉の黄変が目立つ場合は、根の傷みや栄養不足を確認しましょう。",
        "今日は電車が遅れて会社に遅刻しました。",
    ]
    vectors = [embed(text, "passage") for text in passages]
    cases = [("鉢の表面が乾いてきた。そろそろ水をあげるべき？", 0),
             ("葉っぱが黄色く変わってきた", 1)]
    rankings = []
    for query, expected in cases:
        query_vector = embed(query, "query")
        scores = [sum(a * b for a, b in zip(query_vector, vector)) for vector in vectors]
        ranking = sorted(range(len(scores)), key=lambda i: scores[i], reverse=True)
        assert ranking[0] == expected, (ranking, scores)
        rankings.append({"expected_document": expected, "rank": ranking.index(expected) + 1,
                         "cosine_scores": [round(score, 6) for score in scores]})

    # A full context also proves truncation, finite shape and bounded serial inference.
    long_body = {"text": "植物の土と根の状態を詳しく観察します。" * 600, "kind": "passage"}
    with ThreadPoolExecutor(max_workers=1) as pool:
        pending = pool.submit(request, "/embed", long_body)
        busy_seen = False
        for _ in range(200):
            _, state = request("/health")
            if state["inference_busy"]:
                busy_seen = True
                break
            if pending.done():
                break
            time.sleep(0.005)
        assert busy_seen, "Could not observe active full-context inference"
        code, error = request("/embed", {"text": "別の問い合わせ", "kind": "query"})
        assert code == 429 and error["error"] == "inference_busy"
        code, long_result = pending.result()
    assert code == 200 and long_result["truncated"] and long_result["token_count"] == 512
    validate(long_result)

    error_cases = [
        ({"text": "", "kind": "query"}, {}, 400),
        ({"text": "確認", "kind": "invalid"}, {}, 400),
        ({"text": "a" * 66000, "kind": "query"}, {}, 413),
        ({"text": "確認", "kind": "query"}, {"Origin": "https://example.com"}, 403),
        ({"text": "確認", "kind": "query"}, {"Host": "example.com"}, 403),
        ({"text": "確認", "kind": "query"}, {"Content-Type": "text/plain"}, 415),
    ]
    for body, headers, expected in error_cases:
        code, _ = request("/embed", body, headers)
        assert code == expected, (expected, code)
    code, _ = request("/not-an-endpoint")
    assert code == 404
    print(json.dumps({
        "verified": True, "embedding_space": EMBEDDING_SPACE, "dimension": 384,
        "finite": True, "unit_norm": True, "japanese_paraphrase_rankings": rankings,
        "short_request_median_ms": round(statistics.median(timings), 1),
        "short_request_max_ms": round(max(timings), 1),
        "max_tokens": 512, "long_text_truncated": True,
        "concurrent_inference_rejected": True, "bounded_request_checks": True,
    }, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
