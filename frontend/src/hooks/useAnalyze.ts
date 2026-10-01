import { useCallback, useRef, useState } from 'react';
import { isAbort, postForm } from '../lib/api';
import { toForm, type DetectionSettings } from '../lib/settings';
import type { AnalyzeResponse } from '../types/api';

export interface UseAnalyzeResult {
  data: AnalyzeResponse | null;
  loading: boolean;
  error: string | null;
  analyze: () => void;
}

export function useAnalyze(settings: DetectionSettings, ready: boolean): UseAnalyzeResult {
  const [data, setData] = useState<AnalyzeResponse | null>(null);
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
      setData(await postForm<AnalyzeResponse>('/api/analyze', toForm(settings, true), ctl.signal));
    } catch (e) {
      if (!isAbort(e)) setError((e as Error).message);
    } finally {
      if (abortRef.current === ctl) setLoading(false);
    }
  }, []); // stable identity

  return { data, loading, error, analyze };
}
