import { useCallback, useState } from 'react';
import type { Dims, PointPair } from '../types/api';

export interface UseWarpResult {
  warp: (pairs: PointPair[], bDims: Dims, aDims: Dims) => Promise<string | null>;
  loading: boolean;
  error: string | null;
}

export function useWarp(): UseWarpResult {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const warp = useCallback(async (
    pairs: PointPair[],
    bDims: Dims,
    aDims: Dims,
  ): Promise<string | null> => {
    setLoading(true);
    setError(null);

    // Convert relative [0,1] coords to absolute pixel coordinates
    const srcPts = pairs.map(p => [
      Math.round(p.src!.x * bDims.w),
      Math.round(p.src!.y * bDims.h),
    ]);
    const dstPts = pairs.map(p => [
      Math.round(p.dst!.x * aDims.w),
      Math.round(p.dst!.y * aDims.h),
    ]);

    const formData = new FormData();
    formData.append('src_pts', JSON.stringify(srcPts));
    formData.append('dst_pts', JSON.stringify(dstPts));

    try {
      const res = await fetch('/api/warp', { method: 'POST', body: formData });
      if (!res.ok) {
        const json = await res.json() as { error: string };
        throw new Error(json.error || 'Warp failed');
      }
      const blob = await res.blob();
      // Return an object URL for preview; caller is responsible for revoking it
      return URL.createObjectURL(blob);
    } catch (e) {
      setError((e as Error).message);
      return null;
    } finally {
      setLoading(false);
    }
  }, []);

  return { warp, loading, error };
}
