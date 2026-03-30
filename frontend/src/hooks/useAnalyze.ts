import { useCallback, useRef, useState } from 'react';
import type { AnalyzeResponse } from '../types/api';

export interface UseAnalyzeParams {
  strength: number;
  minRegion: number;
  morphSize: number;
  preBlurSigma?: number;    // Gaussian σ before AbsDiff (0=off), default 1.5
  normalizeLuma?: boolean;  // subtract per-image mean luma, default true
  closeSize?: number;       // morphological close kernel (1=off), default 3
  highlightColor?: string;  // hex color like "#ff3c3c", default red
  highlightAlpha?: number;  // overlay opacity 0–1, default 0.55
  ready: boolean;
}

function hexToRgb(hex: string): [number, number, number] {
  const m = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex);
  return m ? [parseInt(m[1], 16), parseInt(m[2], 16), parseInt(m[3], 16)] : [255, 60, 60];
}

export interface UseAnalyzeResult {
  data: AnalyzeResponse | null;
  loading: boolean;
  error: string | null;
  analyze: () => void;
}

export function useAnalyze(params: UseAnalyzeParams): UseAnalyzeResult {
  const [data, setData] = useState<AnalyzeResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const paramsRef = useRef(params);
  paramsRef.current = params;

  const analyze = useCallback(async () => {
    const {
      strength, minRegion, morphSize,
      preBlurSigma, normalizeLuma, closeSize,
      highlightColor, highlightAlpha, ready,
    } = paramsRef.current;
    if (!ready) return;

    abortRef.current?.abort();
    abortRef.current = new AbortController();

    setLoading(true);
    setError(null);

    const [r, g, b] = hexToRgb(highlightColor ?? '#ff3c3c');
    const formData = new FormData();
    formData.append('strength', String(100 - strength));
    formData.append('min_region', String(minRegion));
    formData.append('morph_size', String(morphSize));
    formData.append('pre_blur_sigma', String(preBlurSigma ?? 1.5));
    formData.append('normalize_luma', (normalizeLuma ?? true) ? '1' : '0');
    formData.append('close_size', String(closeSize ?? 3));
    formData.append('highlight_r', String(r));
    formData.append('highlight_g', String(g));
    formData.append('highlight_b', String(b));
    formData.append('highlight_alpha', String(highlightAlpha ?? 0.55));

    try {
      const res = await fetch('/api/analyze', {
        method: 'POST',
        body: formData,
        signal: abortRef.current.signal,
      });
      const json = await res.json();
      if (!res.ok) {
        throw new Error((json as { error: string }).error || 'Analysis failed');
      }
      setData(json as AnalyzeResponse);
    } catch (e) {
      if ((e as Error).name !== 'AbortError') {
        setError((e as Error).message);
      }
    } finally {
      setLoading(false);
    }
  }, []); // stable identity

  return { data, loading, error, analyze };
}
