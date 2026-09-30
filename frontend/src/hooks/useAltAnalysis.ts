import { useCallback, useRef, useState } from 'react';
import { isAbort, postForm } from '../lib/api';
import { toForm, type DetectionSettings } from '../lib/settings';
import type { AlternateResponse } from '../types/api';

export interface UseAltAnalysisResult {
  data: AlternateResponse['images'] | null;
  loading: boolean;
  error: string | null;
  analyze: () => void;
}

/** One request computes every alternate view from a single diff pass. */
export function useAltAnalysis(settings: DetectionSettings, ready: boolean): UseAltAnalysisResult {
  const [data, setData] = useState<AlternateResponse['images'] | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const latest = useRef({ settings, ready });
  latest.current = { settings, ready };

  const analyze = useCallback(async () => {
    const { settings, ready } = latest.current;
    if (!ready) return;

    abortRef.current?.abort();
    const ctl = new AbortController();
    abortRef.current = ctl;
    setLoading(true);
    setError(null);

    try {
      const json = await postForm<AlternateResponse>('/api/analyze/alternate', toForm(settings), ctl.signal);
      setData(json.images);
    } catch (e) {
      if (!isAbort(e)) setError((e as Error).message);
    } finally {
      if (abortRef.current === ctl) setLoading(false);
    }
  }, []);

  return { data, loading, error, analyze };
}
