import { useCallback, useRef, useState } from 'react';
import { fetchBlobUrl, postForm } from '../lib/api';
import type { Dims, Registration, UploadResponse } from '../types/api';

export const MAX_UPLOAD_BYTES = 256 * 1024 * 1024;

export interface UseUploadResult {
  uploadBefore: (file: File) => Promise<boolean>;
  uploadAfter: (file: File) => Promise<boolean>;
  uploadBaseline: (file: File) => Promise<boolean>;
  clearBaselines: () => Promise<void>;
  /** Bump the analysis key after server-side settings change (alignment, warp). */
  refresh: () => void;
  uploadingBefore: boolean;
  uploadingAfter: boolean;
  uploadingBaseline: boolean;
  error: string | null;
  beforeDims: Dims | null;
  afterDims: Dims | null;
  // Object URLs — image data is already in memory, no network fetch on use.
  beforeDisplayUrl: string | null;
  afterDisplayUrl: string | null;
  baselines: number;
  registration: Registration | null;
  ready: boolean;
  // Changes whenever server-side inputs change — analysis tabs re-run on it.
  imageKey: string;
}

/** Decode + warm the browser's image cache so toggling/flipping is instant. */
function preload(url: string) {
  const img = new Image();
  img.src = url;
  img.decode().catch(() => {});
}

export function useUpload(): UseUploadResult {
  const [uploadingBefore, setUploadingBefore] = useState(false);
  const [uploadingAfter, setUploadingAfter] = useState(false);
  const [uploadingBaseline, setUploadingBaseline] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [beforeDims, setBeforeDims] = useState<Dims | null>(null);
  const [afterDims, setAfterDims] = useState<Dims | null>(null);
  const [beforeDisplayUrl, setBeforeDisplayUrl] = useState<string | null>(null);
  const [afterDisplayUrl, setAfterDisplayUrl] = useState<string | null>(null);
  const [baselines, setBaselines] = useState(0);
  const [registration, setRegistration] = useState<Registration | null>(null);
  // Monotonic counter: unlike timestamps it can never collide.
  const [key, setKey] = useState(0);
  const bump = useCallback(() => setKey(k => k + 1), []);
  // A late response from a replaced upload must not overwrite the newer one.
  const seq = useRef({ before: 0, after: 0 });

  const send = useCallback(async (
    side: 'before' | 'after',
    file: File,
    setBusy: (b: boolean) => void,
    setDims: (d: Dims) => void,
    setUrl: (fn: (prev: string | null) => string | null) => void,
  ): Promise<boolean> => {
    if (file.size > MAX_UPLOAD_BYTES) {
      setError(`${file.name} is larger than ${MAX_UPLOAD_BYTES / 1024 / 1024} MB`);
      return false;
    }
    const mine = ++seq.current[side];
    setBusy(true);
    setError(null);
    try {
      const form = new FormData();
      form.append('image', file);
      const json = await postForm<UploadResponse>(`/api/upload/${side}`, form);
      // Fetch the server-rendered (EXIF-rotated, PNG) image once and hold it as a blob URL.
      const url = await fetchBlobUrl(`/api/image/${side}`);
      if (mine !== seq.current[side]) {
        URL.revokeObjectURL(url);
        return false;
      }
      setDims(json.dims);
      setBaselines(json.baselines);
      if (json.registration) setRegistration(json.registration);
      setUrl(prev => {
        if (prev) URL.revokeObjectURL(prev);
        return url;
      });
      preload(url);
      bump();
      return true;
    } catch (e) {
      setError((e as Error).message);
      return false;
    } finally {
      if (mine === seq.current[side]) setBusy(false);
    }
  }, [bump]);

  const uploadBefore = useCallback(
    (file: File) => send('before', file, setUploadingBefore, setBeforeDims, setBeforeDisplayUrl),
    [send],
  );
  const uploadAfter = useCallback(
    (file: File) => send('after', file, setUploadingAfter, setAfterDims, setAfterDisplayUrl),
    [send],
  );

  const uploadBaseline = useCallback(async (file: File) => {
    if (file.size > MAX_UPLOAD_BYTES) {
      setError(`${file.name} is larger than ${MAX_UPLOAD_BYTES / 1024 / 1024} MB`);
      return false;
    }
    setUploadingBaseline(true);
    setError(null);
    try {
      const form = new FormData();
      form.append('image', file);
      const json = await postForm<UploadResponse>('/api/upload/baseline', form);
      setBaselines(json.baselines);
      if (json.registration) setRegistration(json.registration);
      bump();
      return true;
    } catch (e) {
      setError((e as Error).message);
      return false;
    } finally {
      setUploadingBaseline(false);
    }
  }, [bump]);

  const clearBaselines = useCallback(async () => {
    try {
      await postForm('/api/baselines/clear');
      setBaselines(0);
      bump();
    } catch (e) {
      setError((e as Error).message);
    }
  }, [bump]);

  return {
    uploadBefore,
    uploadAfter,
    uploadBaseline,
    clearBaselines,
    refresh: bump,
    uploadingBefore,
    uploadingAfter,
    uploadingBaseline,
    error,
    beforeDims,
    afterDims,
    beforeDisplayUrl,
    afterDisplayUrl,
    baselines,
    registration,
    ready: beforeDisplayUrl !== null && afterDisplayUrl !== null,
    imageKey: String(key),
  };
}
