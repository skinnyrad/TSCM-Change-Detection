import { useCallback, useRef, useState } from 'react';
import type { AltImageResponse, CannyResponse } from '../types/api';

export interface UseAltAnalysisParams {
  strength: number;
  morphSize: number;
  minRegion: number;
  preBlurSigma?: number;
  normalizeLuma?: boolean;
  closeSize?: number;
  ready: boolean;
}

export interface AltAnalysisData {
  diff: string | null;
  subtraction: string | null;
  heatmap: string | null;
  edges: string | null;
  contours: string | null;
}

export interface UseAltAnalysisResult {
  data: AltAnalysisData | null;
  loading: boolean;
  error: string | null;
  analyze: () => void;
}

export function useAltAnalysis(params: UseAltAnalysisParams): UseAltAnalysisResult {
  const [data, setData] = useState<AltAnalysisData | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const paramsRef = useRef(params);
  paramsRef.current = params;

  const analyze = useCallback(async () => {
    const { strength, morphSize, minRegion, preBlurSigma, normalizeLuma, closeSize, ready } = paramsRef.current;
    if (!ready) return;

    abortRef.current?.abort();
    abortRef.current = new AbortController();
    const { signal } = abortRef.current;

    setLoading(true);
    setError(null);

    const buildForm = () => {
      const fd = new FormData();
      fd.append('strength', String(100 - strength));
      fd.append('morph_size', String(morphSize));
      fd.append('min_region', String(minRegion));
      fd.append('pre_blur_sigma', String(preBlurSigma ?? 1.5));
      fd.append('normalize_luma', (normalizeLuma ?? true) ? '1' : '0');
      fd.append('close_size', String(closeSize ?? 3));
      return fd;
    };

    const post = (endpoint: string) =>
      fetch(`/api/analyze/${endpoint}`, { method: 'POST', body: buildForm(), signal });

    try {
      const [diffRes, subRes, heatRes, cannyRes] = await Promise.all([
        post('diff'),
        post('subtraction'),
        post('heatmap'),
        post('canny'),
      ]);

      if (!diffRes.ok || !subRes.ok || !heatRes.ok || !cannyRes.ok) {
        throw new Error('One or more alternate analysis requests failed');
      }

      const [diffJson, subJson, heatJson, cannyJson] = await Promise.all([
        diffRes.json() as Promise<AltImageResponse>,
        subRes.json() as Promise<AltImageResponse>,
        heatRes.json() as Promise<AltImageResponse>,
        cannyRes.json() as Promise<CannyResponse>,
      ]);

      setData({
        diff: diffJson.image ?? null,
        subtraction: subJson.image ?? null,
        heatmap: heatJson.image ?? null,
        edges: cannyJson.edges ?? null,
        contours: cannyJson.contours ?? null,
      });
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
