from __future__ import annotations

import contextlib
import io
import os
import re
import shutil
import threading
from pathlib import Path

_CACHE_DIR = Path(os.getenv("DL_CACHE_DIR", "/tmp/kotonoha-dl-cache"))
_MPLCONFIG_DIR = _CACHE_DIR / "matplotlib"
_XDG_CACHE_DIR = _CACHE_DIR / "xdg"
_MPLCONFIG_DIR.mkdir(parents=True, exist_ok=True)
_XDG_CACHE_DIR.mkdir(parents=True, exist_ok=True)
os.environ.setdefault("MPLBACKEND", "Agg")
os.environ.setdefault("MPLCONFIGDIR", str(_MPLCONFIG_DIR))
os.environ.setdefault("XDG_CACHE_HOME", str(_XDG_CACHE_DIR))

from bonsAI import run_bonsai_diagnosis
from schemas import DiagnosisRatios, DiagnosisResult


_ANALYZE_LOCK = threading.Lock()


class DiagnosisError(RuntimeError):
    pass


def analyze_image(
    *,
    request_id: str,
    image_path: Path,
    work_dir: Path,
    yolo_weights: Path,
) -> DiagnosisResult:
    request_dir = work_dir / request_id
    request_dir.mkdir(parents=True, exist_ok=True)

    input_path = request_dir / image_path.name
    if image_path.resolve() != input_path.resolve():
        shutil.copyfile(image_path, input_path)

    previous_cwd = Path.cwd()
    stdout_buffer = io.StringIO()

    with _ANALYZE_LOCK:
        try:
            os.chdir(request_dir)
            with contextlib.redirect_stdout(stdout_buffer):
                run_bonsai_diagnosis(
                    image_path=str(input_path),
                    yolo_weight=str(yolo_weights.resolve()),
                )
        except Exception as error:
            raise DiagnosisError(str(error)) from error
        finally:
            os.chdir(previous_cwd)

    raw_stdout = stdout_buffer.getvalue()
    health_score, status_label = _parse_stdout(raw_stdout)
    severity_label = _severity_label_for_status(status_label)
    detail = _detail_for_status(status_label, health_score)
    status = _normalize_status(status_label)
    severity = _normalize_severity(severity_label)
    artifacts = _collect_artifacts(request_dir)

    return DiagnosisResult(
        request_id=request_id,
        health_score=health_score,
        status=status,
        status_label=status_label,
        severity=severity,
        severity_label=severity_label,
        detail=detail,
        ratios=DiagnosisRatios(),
        artifacts=artifacts,
        raw_stdout=raw_stdout,
    )


def _parse_stdout(raw_stdout: str) -> tuple[float, str]:
    score_match = re.search(r"総合健康度\s*s\s*:\s*([0-9]+(?:\.[0-9]+)?)", raw_stdout)
    status_match = re.search(r"状態\s*:\s*(.+)", raw_stdout)

    if not score_match:
        message = raw_stdout.strip() or "診断結果を解析できませんでした"
        raise DiagnosisError(message)

    status_label = status_match.group(1).strip() if status_match else "不明"
    return float(score_match.group(1)), status_label


def _detail_for_status(status_label: str, health_score: float) -> str:
    if "不健康" in status_label:
        return f"葉色ヒートマップから、健康度は {health_score:.1f}/100 と判定されました。光・水・根の状態を確認してください。"
    if "健康" in status_label:
        return f"葉色ヒートマップから、健康度は {health_score:.1f}/100 と判定されました。葉色はおおむね安定しています。"
    return f"葉色ヒートマップから、健康度は {health_score:.1f}/100 と判定されました。"


def _severity_label_for_status(status_label: str) -> str:
    if "不健康" in status_label:
        return "高"
    if "健康" in status_label:
        return "なし"
    if "危険" in status_label or "枯死" in status_label or "強い警告" in status_label:
        return "高"
    if "注意" in status_label or "保留" in status_label:
        return "中"
    return "不明"


def _normalize_status(label: str) -> str:
    if "不健康" in label:
        return "warning"
    if "健康" in label:
        return "healthy"
    if "やや注意" in label:
        return "watch"
    if "注意" in label:
        return "warning"
    if "危険" in label or "枯死" in label or "強い警告" in label:
        return "critical"
    return "unknown"


def _normalize_severity(label: str) -> str:
    if label == "なし":
        return "none"
    if "中" in label:
        return "medium"
    if "高" in label:
        return "high"
    return "unknown"


def _collect_artifacts(request_dir: Path) -> dict[str, str]:
    mapping = {
        "heatmap": "output_heatmap.jpg",
    }

    artifacts: dict[str, str] = {}
    for key, filename in mapping.items():
        path = request_dir / filename
        if path.exists():
            artifacts[key] = str(path)
    return artifacts
