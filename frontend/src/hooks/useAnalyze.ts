import { useCallback, useRef, useState } from 'react';
import type { AnalyzeResponse, Method } from '../types/api';

export interface UseAnalyzeParams {
  before: File | null;
  after: File | null;
  method: Method;
  sensitivity: number;
  cannyLow?: number;
  cannyHigh?: number;
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

  // Always-current params ref so `analyze` can be a stable function identity.
  // This prevents the useEffect auto-trigger in tabs from firing on every
  // render just because `analyze` got a new reference.
  const paramsRef = useRef(params);
  paramsRef.current = params;

  const analyze = useCallback(async () => {
    const { before, after, method, sensitivity, cannyLow, cannyHigh } = paramsRef.current;
    if (!before || !after) return;

    // Cancel any in-flight request that was superseded by this call
    abortRef.current?.abort();
    abortRef.current = new AbortController();

    setLoading(true);
    setError(null);

    const formData = new FormData();
    formData.append('before', before);
    formData.append('after', after);
    formData.append('method', method);
    formData.append('sensitivity', String(sensitivity));
    if (cannyLow !== undefined) formData.append('canny_low', String(cannyLow));
    if (cannyHigh !== undefined) formData.append('canny_high', String(cannyHigh));

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
  }, []); // stable — identity never changes

  return { data, loading, error, analyze };
}
