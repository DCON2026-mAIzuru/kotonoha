import warnings
warnings.filterwarnings("ignore", message="Overwriting tiny_vit_.* in registry.*")

import cv2
import numpy as np
import torch
import matplotlib.pyplot as plt
from ultralytics import YOLO
from mobile_sam import sam_model_registry, SamPredictor


# =========================
# 設定
# =========================
CLASS_COLORS = {
    "foliage": np.array([255, 0, 255], dtype=np.uint8),  # BGR: magenta
    "branch":  np.array([0, 128, 255], dtype=np.uint8),  # BGR: orange
    "pot":     np.array([255, 0, 0], dtype=np.uint8),    # BGR: blue
}

LEAF_COLOR_BGR = {
    "green":     np.array([0, 255, 0], dtype=np.uint8),
    "yellow":    np.array([0, 255, 255], dtype=np.uint8),
    "brown_red": np.array([0, 0, 255], dtype=np.uint8),
}

RISK_WEIGHT = {"green": 0.05, "yellow": 0.65, "brown_red": 1.0}


# =========================
# 基本関数
# =========================
def cls_name(model, cls_id):
    return model.names[int(cls_id)]


def resize_mask(mask, image_shape):
    h, w = image_shape[:2]
    return cv2.resize(mask, (w, h), interpolation=cv2.INTER_NEAREST) > 0.5


def clean_mask(mask, k=3):
    u8 = (mask.astype(np.uint8) * 255)
    kernel = np.ones((k, k), np.uint8)
    u8 = cv2.morphologyEx(u8, cv2.MORPH_OPEN, kernel)
    u8 = cv2.morphologyEx(u8, cv2.MORPH_CLOSE, kernel)
    return u8 > 0


def dilate(mask, k=21, it=2):
    kernel = np.ones((k, k), np.uint8)
    return cv2.dilate(mask.astype(np.uint8), kernel, iterations=it) > 0


def erode(mask, k=3, it=1):
    kernel = np.ones((k, k), np.uint8)
    return cv2.erode(mask.astype(np.uint8), kernel, iterations=it) > 0


def overlay_mask(img, mask, color, alpha=0.55):
    out = img.copy()
    out[mask] = (out[mask] * (1 - alpha) + color * alpha).astype(np.uint8)
    return out


# =========================
# 葉色分類
# =========================
def classify_leaf_color(h, s, v):
    if v < 29 or s < 18:
        return None
     # 2) 2枚目のような明るいベージュ背景を除外
    if 8 <= h <= 25 and s <= 80 and v >= 170:
        return None
    if 25 <= h <= 95 and s >= 40 and v >= 40:
        return "green"
    if 12 <= h <= 38 and s >= 40 and v >= 80:
        return "yellow"
    if 5 <= h <= 28 and s >= 30 and v >= 35:
        return "brown_red"
    return None


def ratio_dict(counts):
    total = sum(counts.values())
    if total == 0:
        return {k: 0.0 for k in counts}
    return {k: counts[k] / total * 100 for k in counts}


# =========================
# 画像保存
# =========================
def save_yolo_overlay(img, detections, out_path):
    """YOLO画像：ラベル・ボックスなし。foliage / branch / pot のマスクだけ表示。"""
    out = img.copy()
    for name, mask, _bbox in detections:
        if name in CLASS_COLORS:
            out = overlay_mask(out, mask, CLASS_COLORS[name], alpha=0.50)
    cv2.imwrite(out_path, out)


def save_color_extraction(img, leaf_mask, hsv, out_path):
    """色抽出画像：緑=健康寄り、黄=注意、赤=不健康寄り。"""
    color_map = np.zeros_like(img)
    ys, xs = np.where(leaf_mask)

    for y, x in zip(ys, xs):
        color = classify_leaf_color(*hsv[y, x])
        if color in LEAF_COLOR_BGR:
            color_map[y, x] = LEAF_COLOR_BGR[color]

    out = cv2.addWeighted(img, 0.30, color_map, 0.70, 0)
    out[~leaf_mask] = img[~leaf_mask]
    cv2.imwrite(out_path, out)


def save_heatmap_with_colorbar(img, leaf_mask, risk_map, out_path):
    risk = risk_map.copy()
    risk[~leaf_mask] = 0
    risk = cv2.GaussianBlur(risk, (31, 31), 0)

    if risk[leaf_mask].size and risk[leaf_mask].max() > 0:
        risk = risk / risk[leaf_mask].max()

    img_rgb = cv2.cvtColor(img, cv2.COLOR_BGR2RGB)

    plt.figure(figsize=(8, 6))
    plt.imshow(img_rgb)
    hm = plt.imshow(
        risk,
        cmap="RdYlGn_r",
        alpha=np.where(leaf_mask, 0.55, 0.0),
        vmin=0,
        vmax=1
    )
    cbar = plt.colorbar(hm, fraction=0.046, pad=0.04)
    cbar.set_label("Unhealthy risk  (red = unhealthy)")
    plt.axis("off")
    plt.tight_layout()
    plt.savefig(out_path, dpi=200, bbox_inches="tight")
    plt.close()


# =========================
# SAMをYOLOセグメンテーション形状に寄せる
# =========================
def sam_refine_with_yolo_mask(predictor, bbox, yolo_mask_small, yolo_mask_full,
                              yolo_limit_kernel=21, yolo_limit_iter=2):
    """
    boxだけでSAMに投げるのではなく、
    YOLO segmentationをmask_inputとして渡して形状を寄せる。
    さらにSAMが広がりすぎないように、YOLO foliageを少し膨張した範囲に制限する。
    """
    mask_256 = cv2.resize(yolo_mask_small, (256, 256), interpolation=cv2.INTER_NEAREST)
    sam_input = np.where(mask_256 > 0.5, 10.0, -10.0).astype(np.float32)[None, :, :]

    masks, scores, _ = predictor.predict(
        box=bbox,
        mask_input=sam_input,
        multimask_output=False
    )

    sam_mask = masks[0] > 0.5
    yolo_soft_limit = dilate(yolo_mask_full, k=yolo_limit_kernel, it=yolo_limit_iter)

    refined = sam_mask & yolo_soft_limit
    return refined, float(scores[0])


# =========================
# 領域分割・色解析
# =========================
def split_regions(mask, min_area_abs=50, min_area_ratio=0.002):
    total = int(mask.sum())
    if total == 0:
        return []

    min_area = max(min_area_abs, int(total * min_area_ratio))
    num, labels, stats, _ = cv2.connectedComponentsWithStats(
        mask.astype(np.uint8), connectivity=8
    )

    return [
        labels == i
        for i in range(1, num)
        if stats[i, cv2.CC_STAT_AREA] >= min_area
    ]


def analyze_colors(leaf_mask, hsv):
    zones = {
        "total":  {"green": 0, "yellow": 0, "brown_red": 0},
        "top":    {"green": 0, "yellow": 0, "brown_red": 0},
        "middle": {"green": 0, "yellow": 0, "brown_red": 0},
        "bottom": {"green": 0, "yellow": 0, "brown_red": 0},
    }

    risk_map = np.zeros(leaf_mask.shape, dtype=np.float32)
    region_infos = []

    for rid, region in enumerate(split_regions(leaf_mask), start=1):
        ys, xs = np.where(region)
        if len(ys) == 0:
            continue

        y1, y2 = int(ys.min()), int(ys.max())
        x1, x2 = int(xs.min()), int(xs.max())
        h = max(y2 - y1 + 1, 1)

        r_counts = {"green": 0, "yellow": 0, "brown_red": 0}

        for y, x in zip(ys, xs):
            color = classify_leaf_color(*hsv[y, x])
            if color is None:
                continue

            if y < y1 + h / 3:
                zone, zone_weight = "top", 1.25
            elif y < y1 + h * 2 / 3:
                zone, zone_weight = "middle", 1.0
            else:
                zone, zone_weight = "bottom", 0.9

            zones["total"][color] += 1
            zones[zone][color] += 1
            r_counts[color] += 1
            risk_map[y, x] = min(1.0, RISK_WEIGHT[color] * zone_weight)

        total = sum(r_counts.values())
        if total == 0:
            continue

        rr = ratio_dict(r_counts)
        damage = rr["yellow"] * 0.65 + rr["brown_red"] * 1.0

        region_infos.append({
            "id": rid,
            "bbox": (x1, y1, x2, y2),
            "area": int(region.sum()),
            "green": rr["green"],
            "yellow": rr["yellow"],
            "brown_red": rr["brown_red"],
            "abnormal": rr["yellow"] + rr["brown_red"],
            "damage": damage,
        })

    return {k: ratio_dict(v) for k, v in zones.items()}, risk_map, region_infos


# =========================
# 健康度・判定
# =========================
def calc_health_score(r):
    total, top, mid, bottom = r["total"], r["top"], r["middle"], r["bottom"]

    score = 100
    score -= top["brown_red"] * 2.2 + top["yellow"] * 1.8
    score -= mid["brown_red"] * 1.2 + mid["yellow"] * 0.9
    score -= total["brown_red"] * 0.8 + total["yellow"] * 0.6

    if top["green"] >= 50:
        score -= bottom["brown_red"] * 0.8 + bottom["yellow"] * 0.4
    else:
        score -= bottom["brown_red"] * 1.5 + bottom["yellow"] * 1.0

    return float(np.clip(score, 0, 100))


def judge(r):
    total, top, mid, bottom = r["total"], r["top"], r["middle"], r["bottom"]

    if total["brown_red"] > 35:
        return "枯死に近い危険状態", "高", "茶色の割合が高く、乾燥・枯れ込み・深刻な障害が疑われます。"
    if total["yellow"] > 35:
        return "強い警告", "高", "黄色の割合が高く、根の異常・排水不良・栄養不足などが疑われます。"
    if top["brown_red"] > 10 or top["yellow"] > 7:
        return "危険（上部に異常）", "高", "葉領域上部に黄変・茶変が見られ、進行中のダメージが疑われます。"
    if mid["brown_red"] > 12 or mid["yellow"] > 10:
        return "注意（中央に異常）", "中", "葉領域中央付近に黄変・茶変が見られます。"
    if bottom["brown_red"] > 10 or bottom["yellow"] > 7:
        return "注意（下部に異常）", "中", "葉領域下部に黄変・茶変が見られます。"
    if total["yellow"] > 7:
        return "やや注意", "中", "明確な重症ではありませんが、葉色変化の経過観察が必要です。"
    if total["green"] >= 90:
        return "健康", "なし", "緑が十分に保たれており、全体として良好です。"
    return "判定保留", "低〜中", "明確な異常は限定的ですが、継続観察が必要です。"


def local_warning(region_infos):
    if not region_infos:
        return "目立った局所的異常は検出されませんでした。", None

    worst = max(region_infos, key=lambda x: x["damage"])

    if worst["brown_red"] >= 15:
        text = "一部の葉領域で茶変が集中しており、局所的な枯れ込みの可能性があります。"
    elif worst["damage"] >= 35:
        text = "全体評価が良くても、一部の葉領域に黄変・茶変が集中しています。"
    elif worst["damage"] >= 25:
        text = "一部の葉領域に局所的な黄変が見られます。"
    else:
        text = "目立った局所的異常は検出されませんでした。"

    return text, worst


# =========================
# メイン
# =========================
def analyze_bonsai(
    image_path="bonsai.png",
    yolo_weights="best.pt",
    sam_weights="mobile_sam.pt",
    use_sam=True,
    yolo_conf=0.25,
    yolo_iou=0.5,
    yolo_limit_kernel=21,
    yolo_limit_iter=2,
    ignore_erode_kernel=3,
    ignore_erode_iter=1,
    clean_kernel=3,
    out_prefix="output"
):
    """
    use_sam=True:
        YOLO foliage maskをSAMのmask_inputとして使い、葉マスクを補正する。
    use_sam=False:
        SAMを読み込まず、YOLO segmentation結果だけで葉マスクを作る。
    """
    img = cv2.imread(image_path)
    if img is None:
        raise FileNotFoundError(f"画像を読み込めません: {image_path}")

    yolo = YOLO(yolo_weights)

    predictor = None
    if use_sam:
        device = "cuda" if torch.cuda.is_available() else "cpu"
        sam = sam_model_registry["vit_t"](checkpoint=sam_weights).to(device=device)
        sam.eval()
        predictor = SamPredictor(sam)
        predictor.set_image(cv2.cvtColor(img, cv2.COLOR_BGR2RGB))

    result = yolo(img, conf=yolo_conf, iou=yolo_iou)[0]
    if result.boxes is None or result.masks is None:
        raise RuntimeError("YOLOで foliage / branch / pot が検出されませんでした。")

    boxes = result.boxes.xyxy.cpu().numpy()
    clss = result.boxes.cls.cpu().numpy()
    masks_small = result.masks.data.cpu().numpy()

    detections = []
    foliage_masks = []
    ignore_masks = []

    for bbox, cls_id, small_mask in zip(boxes, clss, masks_small):
        name = cls_name(yolo, cls_id)
        full_mask = resize_mask(small_mask, img.shape)
        detections.append((name, full_mask, bbox))

        if name in ["branch", "pot"]:
            ignore_masks.append(full_mask)
            continue

        if name == "foliage":
            if use_sam:
                refined, _score = sam_refine_with_yolo_mask(
                    predictor,
                    bbox=bbox,
                    yolo_mask_small=small_mask,
                    yolo_mask_full=full_mask,
                    yolo_limit_kernel=yolo_limit_kernel,
                    yolo_limit_iter=yolo_limit_iter
                )
                foliage_masks.append(refined)
            else:
                # SAM OFF時はYOLO segmentationをそのまま葉マスクとして使う
                foliage_masks.append(full_mask)

    if not foliage_masks:
        raise RuntimeError("foliage が検出されませんでした。yolo_confを下げるか、クラス名を確認してください。")

    # 1) YOLO画像：ラベル・ボックスなし
    yolo_path = f"{out_prefix}_1_yolo_overlay_no_label_box.jpg"
    save_yolo_overlay(img, detections, yolo_path)

    # 2) branch/pot除外後の葉マスク画像
    leaf_before_remove = np.max(foliage_masks, axis=0) > 0

    if ignore_masks:
        ignore = np.max(ignore_masks, axis=0) > 0
        # 枝・鉢マスクを少し細らせて、葉の境界を削りすぎない
        ignore = erode(ignore, k=ignore_erode_kernel, it=ignore_erode_iter)
        leaf_mask = leaf_before_remove & (~ignore)
    else:
        leaf_mask = leaf_before_remove

    leaf_mask = clean_mask(leaf_mask, k=clean_kernel)

    if not np.any(leaf_mask):
        raise RuntimeError("branch/pot除外後、葉領域が残りませんでした。")

    method_name = "sam" if use_sam else "yolo_only"
    leaf_path = f"{out_prefix}_2_{method_name}_leaf_after_branch_pot_removed.jpg"
    leaf_vis = overlay_mask(img, leaf_mask, CLASS_COLORS["foliage"], alpha=0.55)
    cv2.imwrite(leaf_path, leaf_vis)

    # 解析
    hsv = cv2.cvtColor(img, cv2.COLOR_BGR2HSV)
    ratios, risk_map, region_infos = analyze_colors(leaf_mask, hsv)

    # 3) 色抽出画像
    color_path = f"{out_prefix}_3_color_extraction.jpg"
    save_color_extraction(img, leaf_mask, hsv, color_path)

    score = calc_health_score(ratios)
    status, severity, detail = judge(ratios)
    local_text, worst = local_warning(region_infos)

    # 4) ヒートマップ：カラーバー付き
    heatmap_path = f"{out_prefix}_4_heatmap_red_unhealthy.png"
    save_heatmap_with_colorbar(img, leaf_mask, risk_map, heatmap_path)

    output = {
        "SAM使用": use_sam,
        "総合健康度": round(score, 1),
        "状態判定": status,
        "詳細": detail,
        "領域ごとの色割合": {
            "全体": ratios["total"],
            "上部": ratios["top"],
            "中央": ratios["middle"],
            "下部": ratios["bottom"],
        },
        "局所注意": local_text,
        "最大危険領域": worst,
        "出力画像": {
            "YOLO画像_ラベルボックスなし": yolo_path,
            "branch_pot除外後葉マスク画像": leaf_path,
            "色抽出画像": color_path,
            "ヒートマップ画像_赤が不健康": heatmap_path,
        }
    }

    print_result(output)
    return output


def print_ratio(label, r):
    print(f"{label}: 緑 {r['green']:.1f}% / 黄 {r['yellow']:.1f}% / 茶 {r['brown_red']:.1f}%")


def print_result(o):
    print("\n========== 盆栽 健康診断結果 ==========")
    print(f"総合健康度: {o['総合健康度']} / 100")
    print(f"状態判定  : {o['状態判定']}")
    print(f"詳細      : {o['詳細']}")
    print("\n--- 領域ごとの色割合 ---")
    for label, r in o["領域ごとの色割合"].items():
        print_ratio(label, r)
    print("\n--- 局所注意 ---")
    print(o["局所注意"])
    print("=======================================")


if __name__ == "__main__":
    analyze_bonsai(
        image_path="image_6.png",
        yolo_weights="best.pt",
        sam_weights="mobile_sam.pt",

        # True : YOLO segmentation + SAM補正
        # False: YOLO segmentationのみ。SAMモデルは読み込まない
        use_sam=True,

        # YOLOの葉領域にSAMを寄せる強さ
        # 大きいほどSAMがYOLO範囲の外側へ少し広がる
        yolo_limit_kernel=7,
        yolo_limit_iter=1,

        # 検出しきれない場合は 0.15〜0.25 に下げる
        yolo_conf=0.25,
        yolo_iou=0.5,

        # 枝・鉢を削りすぎる場合は iterations=1 のまま、kernelを3程度にする
        ignore_erode_kernel=3,
        ignore_erode_iter=1,

        out_prefix="output"
    )
