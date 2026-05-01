'use client';

import { ChangeEvent, useEffect, useMemo, useState } from 'react';
import { getDiagnosisArtifact, getLatestDiagnosis, uploadDiagnosisImage } from '@/lib/backend-api';
import type { BonsaiDiagnosis, DiagnosisColorRatios } from '@/types/api';

interface DiagnosisPanelProps {
  token: string;
  onClose?: () => void;
}

const ARTIFACT_ORDER = ['color_extraction', 'sam_segmentation', 'yolo_detection'] as const;

export function DiagnosisPanel({ token, onClose }: DiagnosisPanelProps) {
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [previewUrl, setPreviewUrl] = useState('');
  const [diagnosis, setDiagnosis] = useState<BonsaiDiagnosis | null>(null);
  const [artifactUrls, setArtifactUrls] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState('');
  const [isError, setIsError] = useState(false);

  const selectedLabel = useMemo(() => {
    if (!selectedFile) return '画像を選択';
    return selectedFile.name.length > 22 ? `${selectedFile.name.slice(0, 20)}...` : selectedFile.name;
  }, [selectedFile]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    getLatestDiagnosis(token)
      .then((latest) => {
        if (!cancelled) {
          setDiagnosis(latest);
          setMessage(latest ? '' : 'まだ画像診断はありません');
          setIsError(false);
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setMessage(error instanceof Error ? error.message : '最新診断の取得に失敗しました');
          setIsError(true);
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [token]);

  useEffect(() => {
    if (!selectedFile) {
      setPreviewUrl('');
      return;
    }
    const url = URL.createObjectURL(selectedFile);
    setPreviewUrl(url);
    return () => URL.revokeObjectURL(url);
  }, [selectedFile]);

  useEffect(() => {
    let cancelled = false;
    const objectUrls: string[] = [];

    async function loadArtifacts() {
      if (!diagnosis?.artifact_urls) {
        setArtifactUrls({});
        return;
      }

      const entries = await Promise.all(
        Object.entries(diagnosis.artifact_urls).map(async ([kind, path]) => {
          if (!path) return null;
          const blob = await getDiagnosisArtifact(token, path);
          const url = URL.createObjectURL(blob);
          objectUrls.push(url);
          return [kind, url] as const;
        }),
      );

      if (!cancelled) {
        setArtifactUrls(Object.fromEntries(entries.filter(Boolean) as Array<[string, string]>));
      }
    }

    loadArtifacts().catch((error) => {
      if (!cancelled) {
        setMessage(error instanceof Error ? error.message : '可視化画像の取得に失敗しました');
        setIsError(true);
      }
    });

    return () => {
      cancelled = true;
      objectUrls.forEach((url) => URL.revokeObjectURL(url));
    };
  }, [diagnosis, token]);

  const onFileChange = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0] ?? null;
    setSelectedFile(file);
    setMessage('');
    setIsError(false);
  };

  const runDiagnosis = async () => {
    if (!selectedFile) {
      setMessage('診断する画像を選択してください');
      setIsError(true);
      return;
    }

    setLoading(true);
    setMessage('');
    setIsError(false);
    try {
      const result = await uploadDiagnosisImage(token, selectedFile);
      setDiagnosis(result);
      setMessage('画像診断が完了しました');
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '画像診断に失敗しました');
      setIsError(true);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="k-push-card" style={{ maxHeight: 'min(760px, calc(100dvh - 48px))', overflow: 'auto' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
        <h3 style={{ margin: 0 }}>画像診断</h3>
        {onClose && <button type="button" className="k-status-chip" onClick={onClose}>閉じる</button>}
      </div>

      <div style={{ display: 'grid', gap: 12 }}>
        <label
          className="k-status-chip"
          style={{ width: 'fit-content', cursor: 'pointer', minHeight: 36, fontSize: '0.82rem' }}
        >
          {selectedLabel}
          <input
            type="file"
            accept="image/*"
            capture="environment"
            onChange={onFileChange}
            style={{ display: 'none' }}
          />
        </label>

        {previewUrl && (
          <img
            src={previewUrl}
            alt="診断する盆栽画像"
            style={{ width: '100%', maxHeight: 220, objectFit: 'cover', borderRadius: 8, border: '1px solid var(--line)' }}
          />
        )}

        <button
          type="button"
          className="k-btn-primary"
          disabled={loading || !selectedFile}
          onClick={() => void runDiagnosis()}
          style={{ maxWidth: 180 }}
        >
          {loading ? '診断中...' : '診断する'}
        </button>

        {message && (
          <p className={isError ? 'k-message-err' : 'k-message-ok'} style={{ margin: 0 }}>
            {message}
          </p>
        )}
      </div>

      {diagnosis && (
        <section style={{ display: 'grid', gap: 14, marginTop: 18 }}>
          <div style={{ display: 'flex', alignItems: 'end', justifyContent: 'space-between', gap: 12 }}>
            <div>
              <p style={{ margin: '0 0 4px', color: 'var(--muted)', fontSize: '0.78rem' }}>総合健康度</p>
              <strong style={{ fontSize: '2rem', lineHeight: 1 }}>{diagnosis.health_score.toFixed(1)}</strong>
              <span style={{ color: 'var(--muted)', marginLeft: 4 }}>/ 100</span>
            </div>
            <div style={{ textAlign: 'right' }}>
              <span className="k-mini-badge">{diagnosis.status_label}</span>
              <p style={{ margin: '6px 0 0', color: 'var(--muted)', fontSize: '0.78rem' }}>
                深刻度: {diagnosis.severity_label}
              </p>
            </div>
          </div>

          <p style={{ margin: 0, color: 'var(--text)', fontSize: '0.9rem', lineHeight: 1.7 }}>
            {diagnosis.detail}
          </p>

          <div style={{ display: 'grid', gap: 8 }}>
            <RatioRow label="全体" ratios={diagnosis.ratios.total} />
            <RatioRow label="新芽" ratios={diagnosis.ratios.top} />
            <RatioRow label="古葉" ratios={diagnosis.ratios.bottom} />
          </div>

          {ARTIFACT_ORDER.some((kind) => artifactUrls[kind]) && (
            <div style={{ display: 'grid', gap: 10 }}>
              {ARTIFACT_ORDER.map((kind) => artifactUrls[kind] ? (
                <figure key={kind} style={{ margin: 0 }}>
                  <img
                    src={artifactUrls[kind]}
                    alt={artifactLabel(kind)}
                    style={{ width: '100%', borderRadius: 8, border: '1px solid var(--line)' }}
                  />
                  <figcaption style={{ marginTop: 4, color: 'var(--muted)', fontSize: '0.76rem' }}>
                    {artifactLabel(kind)}
                  </figcaption>
                </figure>
              ) : null)}
            </div>
          )}
        </section>
      )}
    </div>
  );
}

function RatioRow({ label, ratios }: { label: string; ratios: DiagnosisColorRatios }) {
  return (
    <div style={{ display: 'grid', gap: 4 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.78rem', color: 'var(--muted)' }}>
        <span>{label}</span>
        <span>緑 {ratios.green.toFixed(1)}% / 黄 {ratios.yellow.toFixed(1)}% / 茶 {ratios.brown_red.toFixed(1)}%</span>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: `${Math.max(ratios.green, 1)}fr ${Math.max(ratios.yellow, 1)}fr ${Math.max(ratios.brown_red, 1)}fr`, height: 8, overflow: 'hidden', borderRadius: 999 }}>
        <span style={{ background: '#5e8b61' }} />
        <span style={{ background: '#d5b574' }} />
        <span style={{ background: '#9c6048' }} />
      </div>
    </div>
  );
}

function artifactLabel(kind: string): string {
  switch (kind) {
    case 'color_extraction':
      return '葉色抽出';
    case 'sam_segmentation':
      return 'SAM セグメンテーション';
    case 'yolo_detection':
      return 'YOLO 検出';
    default:
      return kind;
  }
}
