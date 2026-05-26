import { useCallback, useState } from 'react';
import type { AutoWarpResponse, Dims, PointPair } from '../types/api';

export interface AutoWarpPreview {
  pairs: PointPair[];
  confidence: number;
  matchCount: number;
  inlierCount: number;
}

export interface UseAutoWarpResult {
  autoWarp: (bDims: Dims, aDims: Dims) => Promise<AutoWarpPreview | null>;
  loading: boolean;
  error: string | null;
}

export function useAutoWarp(): UseAutoWarpResult {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const autoWarp = useCallback(async (bDims: Dims, aDims: Dims): Promise<AutoWarpPreview | null> => {
    setLoading(true);
    setError(null);

    try {
      const res = await fetch('/api/auto-warp', { method: 'POST' });
      if (!res.ok) {
        const json = await res.json() as { error?: string };
        throw new Error(json.error || 'Auto alignment failed');
      }

      const json = await res.json() as AutoWarpResponse;
      const pairs: PointPair[] = json.pairs.map((pair, index) => ({
        id: index + 1,
        src: { x: pair.src[0] / bDims.w, y: pair.src[1] / bDims.h },
        dst: { x: pair.dst[0] / aDims.w, y: pair.dst[1] / aDims.h },
      }));

      return {
        pairs,
        confidence: json.confidence,
        matchCount: json.match_count,
        inlierCount: json.inlier_count,
      };
    } catch (e) {
      setError((e as Error).message);
      return null;
    } finally {
      setLoading(false);
    }
  }, []);

  return { autoWarp, loading, error };
}