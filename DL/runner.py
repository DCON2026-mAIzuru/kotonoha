from __future__ import annotations

import contextlib
import io
import os
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

from bonsAI import analyze_bonsai
from schemas import ColorRatios, DiagnosisRatios, DiagnosisResult


_ANALYZE_LOCK = threading.Lock()


class DiagnosisError(RuntimeError):
    pass


def analyze_image(
    *,
    request_id: str,
    image_path: Path,
    work_dir: Path,
    yolo_weights: Path,
    sam_weights: Path,
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
                output = analyze_bonsai(
                    image_path=str(input_path),
                    yolo_weights=str(yolo_weights.resolve()),
                    sam_weights=str(sam_weights.resolve()),
                )
        except (FileNotFoundError, RuntimeError) as error:
            raise DiagnosisError(str(error)) from error
        finally:
            os.chdir(previous_cwd)

    raw_stdout = stdout_buffer.getvalue()
    if not isinstance(output, dict) or "総合健康度" not in output:
        raise DiagnosisError("診断結果を解析できませんでした")

    health_score = float(output["総合健康度"])
    status_label = str(output.get("状態判定") or "不明")
    severity_label = _severity_label_for_status(status_label)
    detail = str(output.get("詳細") or "診断の詳細を取得できませんでした。")
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
        ratios=_parse_ratios(output),
        artifacts=artifacts,
        raw_stdout=raw_stdout,
    )


def _parse_ratios(output: dict) -> DiagnosisRatios:
    ratios = output.get("領域ごとの色割合") or {}
    return DiagnosisRatios(
        total=_color_ratios(ratios.get("全体")),
        top=_color_ratios(ratios.get("上部")),
        bottom=_color_ratios(ratios.get("下部")),
    )


def _color_ratios(value: object) -> ColorRatios:
    if not isinstance(value, dict):
        return ColorRatios()
    return ColorRatios(
        green=float(value.get("green", 0.0)),
        yellow=float(value.get("yellow", 0.0)),
        brown_red=float(value.get("brown_red", 0.0)),
    )


def _severity_label_for_status(status_label: str) -> str:
    if "健康" in status_label:
        return "なし"
    if "危険" in status_label or "枯死" in status_label or "強い警告" in status_label:
        return "高"
    if "注意" in status_label or "保留" in status_label:
        return "中"
    return "不明"


def _normalize_status(label: str) -> str:
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
        "yolo_detection": "output_1_yolo_overlay_no_label_box.jpg",
        "sam_segmentation": "output_2_sam_leaf_after_branch_pot_removed.jpg",
        "color_extraction": "output_3_color_extraction.jpg",
    }

    artifacts: dict[str, str] = {}
    for key, filename in mapping.items():
        path = request_dir / filename
        if path.exists():
            artifacts[key] = str(path)
    return artifacts
