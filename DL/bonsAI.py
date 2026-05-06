import warnings
# 警告の非表示を一番最初に行う
warnings.filterwarnings("ignore")
import cv2
import numpy as np
import torch
import matplotlib.pyplot as plt
import matplotlib.colors as mcolors  # カラーバー作成のために追加
from ultralytics import YOLO
import japanize_matplotlib

def risk_to_bgr(risk):
    """
    リスクスコア(0.0-1.0)をBGR色に変換する
    0.0 = 緑, 0.5 = 黄, 1.0 = 赤
    """
    risk = np.clip(risk, 0.0, 1.0)
    if risk < 0.5:
        t = risk / 0.5
        return (0, 255, int(255 * t))
    else:
        t = (risk - 0.5) / 0.5
        return (0, int(255 * (1.0 - t)), 255)

def run_bonsai_diagnosis(image_path, yolo_weight="best.pt"):
    # 1. 準備とモデルのロード
    yolo_model = YOLO(yolo_weight)

    # 画像の読み込み
    img = cv2.imread(image_path)
    if img is None:
        print("画像が読み込めません。")
        return
    
    hsv_img = cv2.cvtColor(img, cv2.COLOR_BGR2HSV_FULL)
    h_orig, w_orig = img.shape[:2]

    # 2. YOLOによる推定とマスク抽出
    results = yolo_model(img, conf=0.25)[0]
    
    class_names = yolo_model.names
    name_to_id = {v: k for k, v in class_names.items()}
    
    foliage_id = name_to_id.get('foliage')
    branch_id = name_to_id.get('branch')
    pot_id = name_to_id.get('pot')

    exclude_mask = np.zeros((h_orig, w_orig), dtype=np.uint8)
    yolo_foliage_mask = np.zeros((h_orig, w_orig), dtype=np.uint8)
    foliage_detected = False

    if results.masks is not None:
        for mask, box in zip(results.masks.data, results.boxes):
            cls = int(box.cls[0])
            m = (cv2.resize(mask.cpu().numpy(), (w_orig, h_orig)) > 0.5).astype(np.uint8)
            
            if cls == foliage_id:
                # YOLOが予測した葉のマスクを合成
                yolo_foliage_mask = cv2.bitwise_or(yolo_foliage_mask, m)
                foliage_detected = True
            elif cls == branch_id or cls == pot_id:
                exclude_mask = cv2.bitwise_or(exclude_mask, m)

    if not foliage_detected:
        print("葉(foliage)が検出されませんでした。")
        return

    # 3. 領域の差し引き (YOLOの葉マスクから枝と鉢を除外)
    final_leaf_mask = cv2.bitwise_and(yolo_foliage_mask, cv2.bitwise_not(exclude_mask))
    
    # 4. ルールベース処理 (s1, s2 の計算)
    y_indices, x_indices = np.where(final_leaf_mask > 0)
    if len(y_indices) == 0:
        print("有効な葉のピクセルが存在しません。")
        return

    min_y_foliage, max_y_foliage = np.min(y_indices), np.max(y_indices)
    height_range = max_y_foliage - min_y_foliage

    scores_c = []
    scores_cu = []
    scores_cb = []
    heatmap_c = np.zeros((h_orig, w_orig), dtype=np.float32)

    # 除外ピクセルを管理するためのマスクを初期化
    valid_foliage_mask = np.zeros((h_orig, w_orig), dtype=np.uint8)

    for y, x in zip(y_indices, x_indices):
        h_val, s_val, v_val = hsv_img[y, x]
        h_val = h_val / 255 * 360 
        s_val = s_val / 255 * 360
        v_val = v_val / 255 * 360
        
        # 背景ノイズ除外：ここで continue すると以下の計算もヒートマップ用フラグもスキップされる
        if s_val <= 45 and v_val >= 120: continue
        if 8 <= h_val <= 25 and s_val <= 80 and v_val >= 170: continue
        if 12 <= h_val <= 28 and 35 <= s_val <= 90 and 80 <= v_val <= 170: continue
        if v_val <= 29: continue

        # 有効な葉ピクセルとして記録
        valid_foliage_mask[y, x] = 1

        if v_val > 29 and s_val > 18 and 5 < h_val < 95:
            # --- スコア計算ロジック ---
            c = (80 - h_val) / 55
            if c < 0: c = 0
            c = max(0.0, min(1.0, c))
            
            scores_c.append(c)
            heatmap_c[y, x] = c
            
            # --- 部位別計算 ---
            pixel_h = 1.0 - (y - min_y_foliage) / max(height_range, 1)
            if pixel_h > 0.7:
                scores_cu.append(c)
            elif pixel_h < 0.3:
                scores_cb.append(c)

    # スコア計算
    s1 = np.mean(scores_c) if scores_c else 0
    # 総合スコア s
    s = max(0, min(100, (1 - (s1 )) * 100))

    threshold = 58.37
    health_status = "健康" if s >= threshold else "不健康"

    # 5. ヒートマップ出力 
    overlay = img.copy()
    alpha = 0.7
    for y, x in zip(y_indices, x_indices):
        # valid_foliage_mask が 1 のピクセルのみヒートマップを描画する
        if valid_foliage_mask[y, x] > 0:
            c_val = heatmap_c[y, x]
            color = risk_to_bgr(c_val)
            overlay[y, x] = (img[y, x] * (1 - alpha) + np.array(color) * alpha).astype(np.uint8)

    # --- Matplotlibを利用してカラーバー付きの画像を保存 ---
    # 日本語フォントが設定されていない環境での文字化けを防ぐため、PCの標準フォントを使用するよう設定

    # BGRからRGBに変換 (matplotlibはRGBで表示するため)
    overlay_rgb = cv2.cvtColor(overlay, cv2.COLOR_BGR2RGB)
    
    fig, ax = plt.subplots(figsize=(10, 8))
    ax.imshow(overlay_rgb)
    ax.axis('off') # 画像の軸（目盛り）を非表示
    
    # カラーマップの作成 (緑:0.0 -> 黄:0.5 -> 赤:1.0)
    cmap = mcolors.LinearSegmentedColormap.from_list("risk_cmap", ["#00FF00", "#FFFF00", "#FF0000"])
    norm = plt.Normalize(vmin=0, vmax=1)
    sm = plt.cm.ScalarMappable(cmap=cmap, norm=norm)
    sm.set_array([])
    
    # カラーバーを右側に追加
    cbar = fig.colorbar(sm, ax=ax, fraction=0.046, pad=0.04)
    cbar.set_ticks([0.0, 0.5, 1.0])
    # ラベルの設定
    cbar.set_ticklabels(['0.0\n(健康)', '0.5', '1.0\n(不健康)'])
    
    # 保存
    plt.savefig("output_heatmap.jpg", bbox_inches='tight', dpi=150)
    plt.close()

    # 6. 結果の出力
    print(f"  --------------------------")
    print(f"  総合健康度 s: {s:.2f}")
    print(f"  状態         : {health_status}")
    print("診断結果画像を保存しました。")

if __name__ == "__main__":
    run_bonsai_diagnosis("スクリーンショット 2026-05-06 195900.png")