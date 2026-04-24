import warnings

warnings.filterwarnings(
    "ignore",
    message="Overwriting tiny_vit_.* in registry.*"
)
import cv2
import numpy as np
import torch
from ultralytics import YOLO
from mobile_sam import sam_model_registry, SamPredictor
import os


def classify_leaf_color(h, s, v):
    
    if v < 25 or s < 18:
        return None, None

    if 25 <= h <= 95 and s >= 40 and v >= 40:
        return 'green', (0, 255, 0)

    if 12 <= h <= 38 and s >= 40 and v >= 80:
        return 'yellow', (0, 255, 255)

    if 5 <= h <= 28 and s >= 30 and v >= 35:
        return 'brown_red', (0, 0, 255)

    return None, None

def analyze_health_advanced(image_path, yolo_weights='best.pt', sam_weights='mobile_sam.pt'):
    print(f"[{image_path}] の解析を開始します... (Advanced Mode)")

    img = cv2.imread(image_path)
    if img is None:
        print("画像の読み込みに失敗しました。")
        return
    img_rgb = cv2.cvtColor(img, cv2.COLOR_BGR2RGB)

    device = "cuda" if torch.cuda.is_available() else "cpu"
    yolo_model = YOLO(yolo_weights)
    mobile_sam = sam_model_registry["vit_t"](checkpoint=sam_weights).to(device=device)
    mobile_sam.eval()
    predictor = SamPredictor(mobile_sam)

    print("Step 1: YOLOで葉の領域(Box)と初期セグメンテーション(Mask)を検出中...")
    results = yolo_model(img)
    predictor.set_image(img_rgb)

    foliage_masks = []
    img_yolo_viz = img.copy()

    if results[0].boxes is not None and results[0].masks is not None:
        bboxes = results[0].boxes.xyxy.cpu().numpy()
        classes = results[0].boxes.cls.cpu().numpy()
        yolo_masks = results[0].masks.data.cpu().numpy()

        for bbox, cls_id, yolo_mask in zip(bboxes, classes, yolo_masks):
            class_name = yolo_model.names[int(cls_id)]

            if class_name == 'foliage':
                x1, y1, x2, y2 = map(int, bbox)
                cv2.rectangle(img_yolo_viz, (x1, y1), (x2, y2), (255, 0, 0), 2)
                cv2.putText(img_yolo_viz, "YOLO Foliage", (x1, max(y1 - 10, 0)),
                            cv2.FONT_HERSHEY_SIMPLEX, 0.7, (255, 0, 0), 2)

                h_orig, w_orig = img.shape[:2]
                yolo_mask_orig = cv2.resize(yolo_mask, (w_orig, h_orig), interpolation=cv2.INTER_NEAREST)
                overlay_color_yolo = np.array([255, 0, 0])
                img_yolo_viz[yolo_mask_orig > 0.5] = (
                    img_yolo_viz[yolo_mask_orig > 0.5] * 0.5 + overlay_color_yolo * 0.5
                )

                print("Step 2: YOLOのBoxとMaskを組み合わせてSAMへ送信中...")
                mask_256 = cv2.resize(yolo_mask, (256, 256), interpolation=cv2.INTER_NEAREST)
                sam_mask_input = np.where(mask_256 > 0.5, 10.0, -10.0).astype(np.float32)
                sam_mask_input = np.expand_dims(sam_mask_input, axis=0)

                masks, scores, _ = predictor.predict(
                    box=bbox,
                    mask_input=sam_mask_input,
                    multimask_output=False
                )
                foliage_masks.append(masks[0])

    cv2.imwrite("output_1_yolo_detection.jpg", img_yolo_viz)
    print(" -> 'output_1_yolo_detection.jpg' を保存しました。")

    if not foliage_masks:
        print("画像内に Foliage（葉）が検出されませんでした。")
        return

    combined_mask = np.max(foliage_masks, axis=0) > 0.5

    img_sam = img.copy()
    overlay_color = np.array([255, 0, 255])
    img_sam[combined_mask] = img_sam[combined_mask] * 0.5 + overlay_color * 0.5
    cv2.imwrite("output_2_advanced_sam_segmentation.jpg", img_sam)
    print(" -> 'output_2_advanced_sam_segmentation.jpg' を保存しました。")

    # ==========================================
    # 3. 色抽出と領域ごとの集計
    # ==========================================
    print("Step 3: 切り取った葉の色の抽出・解析中...")
    y_indices, x_indices = np.where(combined_mask)
    min_y, max_y = np.min(y_indices), np.max(y_indices)
    height = max_y - min_y
    y_top_limit = min_y + height // 3
    y_bottom_limit = min_y + (height * 2) // 3

    hsv_img = cv2.cvtColor(img, cv2.COLOR_BGR2HSV)

    stats = {
        'total': {'green': 0, 'yellow': 0, 'brown_red': 0},
        'top':   {'green': 0, 'yellow': 0, 'brown_red': 0},
        'bottom':{'green': 0, 'yellow': 0, 'brown_red': 0}
    }

    img_color_viz = np.zeros_like(img)

    for y, x in zip(y_indices, x_indices):
        h, s, v = hsv_img[y, x]
        color_name, viz_bgr = classify_leaf_color(h, s, v)

        if color_name:
            img_color_viz[y, x] = viz_bgr
            stats['total'][color_name] += 1
            if y < y_top_limit:
                stats['top'][color_name] += 1
            elif y >= y_bottom_limit:
                stats['bottom'][color_name] += 1

    blended_color_viz = cv2.addWeighted(img, 0.3, img_color_viz, 0.7, 0)
    cv2.line(blended_color_viz, (0, y_top_limit), (img.shape[1], y_top_limit), (255, 255, 255), 1)
    cv2.line(blended_color_viz, (0, y_bottom_limit), (img.shape[1], y_bottom_limit), (255, 255, 255), 1)
    cv2.imwrite("output_3_advanced_color_extraction.jpg", blended_color_viz)
    print(" -> 'output_3_advanced_color_extraction.jpg' を保存しました。")

    # ==========================================
    # 4. 計算と総合健康度(0~100)の算出
    # ==========================================
    def calculate_ratios(zone_stats):
        total = sum(zone_stats.values())
        if total == 0:
            return {k: 0.0 for k in zone_stats}
        return {k: (v / total * 100) for k, v in zone_stats.items()}

    total_r = calculate_ratios(stats['total'])
    top_r = calculate_ratios(stats['top'])
    bottom_r = calculate_ratios(stats['bottom'])

    # --- 健康度スコア算出ロジック ---
    score = 100.0

    # 新芽（上部）の異常はかなり重く減点
    score -= top_r['brown_red'] * 2.2
    score -= top_r['yellow'] * 1.8

    # 全体の黄変・茶変も追加で減点
    score -= total_r['brown_red'] * 0.8
    score -= total_r['yellow'] * 0.6

    # 古葉（下部）の異常の評価
    if top_r['green'] >= 50:
        # 新芽が元気でも、黄・茶は今までより強めに減点
        score -= bottom_r['brown_red'] * 0.8
        score -= bottom_r['yellow'] * 0.4
    else:
        # 新芽も弱っている場合はかなり大きく減点
        score -= bottom_r['brown_red'] * 1.5
        score -= bottom_r['yellow'] * 1.0

    final_score = max(0.0, min(100.0, score))

    # ==========================================
    # 5. 最終結果の出力
    # ==========================================
    print("\n" + "="*40)
    print("           盆栽 健康診断結果")
    print("="*40)
    print(f"【 総合健康度 】 {final_score:.1f} / 100 点")
    print("-" * 40)
    print(f"[ 全体 ] 緑:{total_r['green']:.1f}% 黄:{total_r['yellow']:.1f}% 茶:{total_r['brown_red']:.1f}%")
    print(f"[ 新芽 ] 緑:{top_r['green']:.1f}% 黄:{top_r['yellow']:.1f}% 茶:{top_r['brown_red']:.1f}%")
    print(f"[ 古葉 ] 緑:{bottom_r['green']:.1f}% 黄:{bottom_r['yellow']:.1f}% 茶:{bottom_r['brown_red']:.1f}%")
    print("-" * 40)

    if total_r['brown_red'] > 35:
        print("【状態判定】枯死に近い危険状態\n【深刻度】高\n【詳細】茶色の割合が高く、乾燥・枯れ込み・深刻な障害が強く疑われます。")
    elif total_r['yellow'] > 35:
        print("【状態判定】強い警告（根の異常・栄養不足）\n【深刻度】高\n【詳細】黄色の割合が高く、根の異常、排水不良、栄養不足などの可能性があります。")
    elif top_r['brown_red'] > 10 or top_r['yellow'] > 7:
        print("【状態判定】危険（新芽に異常）\n【深刻度】高\n【詳細】新芽側に黄変・茶変が見られ、進行中のダメージが疑われます。")
    elif bottom_r['brown_red'] > 10 or bottom_r['yellow'] > 7:
        print("【状態判定】注意\n【深刻度】中\n【詳細】古葉側に黄変・茶変が見られ、弱りの兆候が出ています。")
    elif total_r['yellow'] > 7:
        print("【状態判定】やや注意\n【深刻度】中\n【詳細】明確な重症ではないものの、葉色の変化に注意が必要です。")
    elif top_r['green'] >= 90:
        print("【状態判定】健康\n【深刻度】なし\n【詳細】新芽の緑が十分に保たれており、全体として良好です。")
    print("="*40)

if __name__ == "__main__":
    analyze_health_advanced(image_path="istockphoto-156381110-612x612.jpg")