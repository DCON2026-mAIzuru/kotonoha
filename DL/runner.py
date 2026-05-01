from __future__ import annotations

import contextlib
import io
import os
import re
import shutil
import threading
from pathlib import Path

from bonsAI import analyze_health_advanced
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
                analyze_health_advanced(
                    image_path=str(input_path),
                    yolo_weights=str(yolo_weights.resolve()),
                    sam_weights=str(sam_weights.resolve()),
                )
        finally:
            os.chdir(previous_cwd)

    raw_stdout = stdout_buffer.getvalue()
    if "画像の読み込みに失敗しました" in raw_stdout:
        raise DiagnosisError("画像の読み込みに失敗しました")
    if "画像内に Foliage" in raw_stdout:
        raise DiagnosisError("葉の領域を検出できませんでした")

    health_score = _parse_health_score(raw_stdout)
    if health_score is None:
        raise DiagnosisError("診断結果を解析できませんでした")

    status_label, severity_label, detail = _parse_status(raw_stdout)
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
        ratios=_parse_ratios(raw_stdout),
        artifacts=artifacts,
        raw_stdout=raw_stdout,
    )


def _parse_health_score(stdout: str) -> float | None:
    match = re.search(r"【\s*総合健康度\s*】\s*([0-9]+(?:\.[0-9]+)?)\s*/\s*100", stdout)
    if not match:
        return None
    return float(match.group(1))


def _parse_ratios(stdout: str) -> DiagnosisRatios:
    return DiagnosisRatios(
        total=_parse_ratio_line(stdout, "全体"),
        top=_parse_ratio_line(stdout, "新芽"),
        bottom=_parse_ratio_line(stdout, "古葉"),
    )


def _parse_ratio_line(stdout: str, label: str) -> ColorRatios:
    pattern = rf"\[\s*{re.escape(label)}\s*\]\s*緑:([0-9.]+)%\s*黄:([0-9.]+)%\s*茶:([0-9.]+)%"
    match = re.search(pattern, stdout)
    if not match:
        return ColorRatios()
    return ColorRatios(
        green=float(match.group(1)),
        yellow=float(match.group(2)),
        brown_red=float(match.group(3)),
    )


def _parse_status(stdout: str) -> tuple[str, str, str]:
    status = _parse_bracket_value(stdout, "状態判定") or "不明"
    severity = _parse_bracket_value(stdout, "深刻度") or "不明"
    detail = _parse_bracket_value(stdout, "詳細") or "診断の詳細を取得できませんでした。"
    return status, severity, detail


def _parse_bracket_value(stdout: str, label: str) -> str | None:
    match = re.search(rf"【{re.escape(label)}】([^\n]+)", stdout)
    if not match:
        return None
    return match.group(1).strip()


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
        "yolo_detection": "output_1_yolo_detection.jpg",
        "sam_segmentation": "output_2_advanced_sam_segmentation.jpg",
        "color_extraction": "output_3_advanced_color_extraction.jpg",
    }

    artifacts: dict[str, str] = {}
    for key, filename in mapping.items():
        path = request_dir / filename
        if path.exists():
            artifacts[key] = str(path)
    return artifacts

