import { useCallback, useState } from 'react';
import type { Dims } from '../types/api';

export interface UseUploadResult {
  uploadBefore: (file: File) => Promise<boolean>;
  uploadAfter: (file: File) => Promise<boolean>;
  uploadingBefore: boolean;
  uploadingAfter: boolean;
  beforeDims: Dims | null;
  afterDims: Dims | null;
  // Object URLs — image data is already in memory, no network fetch on use.
  beforeDisplayUrl: string | null;
  afterDisplayUrl: string | null;
  ready: boolean;
  // Changes on every upload — used by analysis tabs to detect when to re-run.
  imageKey: string;
}

export function useUpload(): UseUploadResult {
  const [uploadingBefore, setUploadingBefore] = useState(false);
  const [uploadingAfter, setUploadingAfter] = useState(false);
  const [beforeDims, setBeforeDims] = useState<Dims | null>(null);
  const [afterDims, setAfterDims] = useState<Dims | null>(null);
  const [beforeDisplayUrl, setBeforeDisplayUrl] = useState<string | null>(null);
  const [afterDisplayUrl, setAfterDisplayUrl] = useState<string | null>(null);
  // Timestamps used only for imageKey — changed on each successful upload.
  const [beforeTs, setBeforeTs] = useState(0);
  const [afterTs, setAfterTs] = useState(0);

  const uploadBefore = useCallback(async (file: File): Promise<boolean> => {
    setUploadingBefore(true);
    try {
      // 1. Upload the file — backend stores it and computes alignment.
      const form = new FormData();
      form.append('image', file);
      const uploadRes = await fetch('/api/upload/before', { method: 'POST', body: form });
      const json = await uploadRes.json() as { dims?: Dims; error?: string };
      if (!uploadRes.ok) throw new Error(json.error ?? 'Upload failed');
      setBeforeDims(json.dims!);

      // 2. Fetch the server-rendered PNG exactly once and hold it as an object URL.
      //    All components read from this in-memory blob — no further network requests.
      const imgRes = await fetch('/api/image/before');
      if (!imgRes.ok) throw new Error('Failed to fetch image');
      const url = URL.createObjectURL(await imgRes.blob());
      setBeforeDisplayUrl(prev => { if (prev) URL.revokeObjectURL(prev); return url; });
      setBeforeTs(Date.now());
      return true;
    } catch {
      return false;
    } finally {
      setUploadingBefore(false);
    }
  }, []);

  const uploadAfter = useCallback(async (file: File): Promise<boolean> => {
    setUploadingAfter(true);
    try {
      const form = new FormData();
      form.append('image', file);
      const uploadRes = await fetch('/api/upload/after', { method: 'POST', body: form });
      const json = await uploadRes.json() as { dims?: Dims; error?: string };
      if (!uploadRes.ok) throw new Error(json.error ?? 'Upload failed');
      setAfterDims(json.dims!);

      const imgRes = await fetch('/api/image/after');
      if (!imgRes.ok) throw new Error('Failed to fetch image');
      const url = URL.createObjectURL(await imgRes.blob());
      setAfterDisplayUrl(prev => { if (prev) URL.revokeObjectURL(prev); return url; });
      setAfterTs(Date.now());
      return true;
    } catch {
      return false;
    } finally {
      setUploadingAfter(false);
    }
  }, []);

  return {
    uploadBefore,
    uploadAfter,
    uploadingBefore,
    uploadingAfter,
    beforeDims,
    afterDims,
    beforeDisplayUrl,
    afterDisplayUrl,
    ready: beforeDisplayUrl !== null && afterDisplayUrl !== null,
    imageKey: `${beforeTs}-${afterTs}`,
  };
}
