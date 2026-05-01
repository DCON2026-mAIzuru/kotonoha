from __future__ import annotations

import os
import shutil
import tempfile
from pathlib import Path

from fastapi import FastAPI, File, Form, HTTPException, UploadFile
from fastapi.responses import FileResponse

from runner import DiagnosisError, analyze_image


APP_DIR = Path(__file__).resolve().parent
WORK_DIR = Path(os.getenv("DL_WORK_DIR", "/tmp/kotonoha-dl"))
YOLO_WEIGHTS = Path(os.getenv("DL_YOLO_WEIGHTS", str(APP_DIR / "best.pt")))
SAM_WEIGHTS = Path(os.getenv("DL_SAM_WEIGHTS", str(APP_DIR / "mobile_sam.pt")))
MAX_IMAGE_MB = int(os.getenv("DL_MAX_IMAGE_MB", "8"))
ALLOWED_CONTENT_TYPES = {"image/jpeg", "image/png", "image/webp"}
ARTIFACT_NAMES = {
    "yolo_detection": "output_1_yolo_detection.jpg",
    "sam_segmentation": "output_2_advanced_sam_segmentation.jpg",
    "color_extraction": "output_3_advanced_color_extraction.jpg",
}

app = FastAPI(title="Kotonoha DL Service")


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/v1/diagnose")
def diagnose(
    request_id: str = Form(...),
    image: UploadFile = File(...),
):
    if image.content_type not in ALLOWED_CONTENT_TYPES:
        raise HTTPException(status_code=400, detail="jpg/png/webp 画像を指定してください")
    if not YOLO_WEIGHTS.exists():
        raise HTTPException(status_code=500, detail=f"YOLO重みが見つかりません: {YOLO_WEIGHTS}")
    if not SAM_WEIGHTS.exists():
        raise HTTPException(status_code=500, detail=f"SAM重みが見つかりません: {SAM_WEIGHTS}")

    WORK_DIR.mkdir(parents=True, exist_ok=True)
    suffix = _suffix_for_content_type(image.content_type)
    max_bytes = MAX_IMAGE_MB * 1024 * 1024

    with tempfile.NamedTemporaryFile(delete=False, suffix=suffix) as tmp:
        written = 0
        try:
            while True:
                chunk = image.file.read(1024 * 1024)
                if not chunk:
                    break
                written += len(chunk)
                if written > max_bytes:
                    raise HTTPException(status_code=413, detail=f"画像は {MAX_IMAGE_MB}MB 以下にしてください")
                tmp.write(chunk)
        finally:
            image.file.close()

    tmp_path = Path(tmp.name)
    try:
        return analyze_image(
            request_id=request_id,
            image_path=tmp_path,
            work_dir=WORK_DIR,
            yolo_weights=YOLO_WEIGHTS,
            sam_weights=SAM_WEIGHTS,
        )
    except DiagnosisError as error:
        raise HTTPException(status_code=422, detail=str(error)) from error
    except Exception as error:
        raise HTTPException(status_code=500, detail="画像診断に失敗しました") from error
    finally:
        tmp_path.unlink(missing_ok=True)


@app.get("/v1/artifacts/{request_id}/{kind}")
def artifact(request_id: str, kind: str):
    filename = ARTIFACT_NAMES.get(kind)
    if not filename:
        raise HTTPException(status_code=404, detail="artifact が見つかりません")

    path = WORK_DIR / request_id / filename
    if not path.exists():
        raise HTTPException(status_code=404, detail="artifact が見つかりません")

    return FileResponse(path, media_type="image/jpeg", filename=filename)


def _suffix_for_content_type(content_type: str) -> str:
    if content_type == "image/png":
        return ".png"
    if content_type == "image/webp":
        return ".webp"
    return ".jpg"

