import { useCallback, useState } from 'react';
import type { SyntheticEvent } from 'react';

/** Tracks an <img>'s natural aspect ratio so its box can be sized before layout settles. */
export function useAspectRatio() {
  const [aspectRatio, setAspectRatio] = useState<number | null>(null);
  const onLoad = useCallback((e: SyntheticEvent<HTMLImageElement>) => {
    const { naturalWidth, naturalHeight } = e.currentTarget;
    if (naturalWidth > 0 && naturalHeight > 0) setAspectRatio(naturalWidth / naturalHeight);
  }, []);
  return { aspectRatio, onLoad };
}

/** CSS width that fits an image of the given ratio inside maxHeight and its container. */
export const fitWidth = (aspectRatio: number | null, maxHeight: string) =>
  aspectRatio ? `min(calc(${maxHeight} * ${aspectRatio}), 100%)` : '100%';
