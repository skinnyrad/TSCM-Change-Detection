import { useCallback, useEffect, useRef, useState } from 'react';
import { ApiError, postForm } from '../lib/api';
import type { BatchJob, BatchResults, BatchStatus } from '../types/api';

const CHUNK = 8; // files per upload request

export interface UseBatchResult {
  count: number;
  job: BatchJob;
  results: BatchResults | null;
  uploading: { done: number; total: number } | null;
  error: string | null;
  failed: string[];
  upload: (files: File[]) => Promise<void>;
  clear: () => Promise<void>;
  analyze: () => Promise<void>;
}

const IDLE: BatchJob = { state: 'idle', done: 0, total: 0, duration_s: 0 };

export function useBatch(): UseBatchResult {
  const [count, setCount] = useState(0);
  const [job, setJob] = useState<BatchJob>(IDLE);
  const [results, setResults] = useState<BatchResults | null>(null);
  const [uploading, setUploading] = useState<{ done: number; total: number } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [failed, setFailed] = useState<string[]>([]);
  const poll = useRef<number | null>(null);

  const fetchResults = useCallback(async () => {
    const res = await fetch('/api/batch/results');
    if (res.ok) setResults(await res.json() as BatchResults);
  }, []);

  const refresh = useCallback(async () => {
    const res = await fetch('/api/batch');
    if (!res.ok) return;
    const st = await res.json() as BatchStatus;
    setCount(st.items.length);
    setJob(st.job);
    return st;
  }, []);

  const stopPolling = () => {
    if (poll.current !== null) window.clearInterval(poll.current);
    poll.current = null;
  };

  const startPolling = useCallback(() => {
    stopPolling();
    poll.current = window.setInterval(async () => {
      const st = await refresh().catch(() => undefined);
      if (!st || st.job.state === 'running') return;
      stopPolling();
      if (st.job.state === 'done') await fetchResults();
      if (st.job.state === 'error') setError(st.job.error ?? 'Analysis failed');
    }, 400);
  }, [refresh, fetchResults]);

  // Pick up an existing batch (e.g. after a page reload).
  useEffect(() => {
    refresh().then(st => {
      if (st?.job.state === 'running') startPolling();
      else if (st?.has_results) fetchResults();
    }).catch(() => {});
    return stopPolling;
  }, [refresh, startPolling, fetchResults]);

  const upload = useCallback(async (files: File[]) => {
    const images = files.filter(f => /\.(jpe?g|png)$/i.test(f.name) || /^image\/(jpeg|png)$/.test(f.type));
    if (images.length === 0) {
      setError('No JPG or PNG files found');
      return;
    }
    setError(null);
    setFailed([]);
    setResults(null);
    setUploading({ done: 0, total: images.length });
    try {
      for (let i = 0; i < images.length; i += CHUNK) {
        const fd = new FormData();
        for (const f of images.slice(i, i + CHUNK)) fd.append('images', f, f.webkitRelativePath || f.name);
        const json = await postForm<{ count: number; failed?: { name: string; error: string }[] }>('/api/batch/images', fd);
        setCount(json.count);
        if (json.failed?.length) setFailed(prev => [...prev, ...json.failed!.map(f => `${f.name}: ${f.error}`)]);
        setUploading({ done: Math.min(i + CHUNK, images.length), total: images.length });
      }
      setJob(IDLE);
    } catch (e) {
      setError((e as ApiError).message);
    } finally {
      setUploading(null);
    }
  }, []);

  const clear = useCallback(async () => {
    try {
      await postForm('/api/batch/clear');
      setCount(0);
      setResults(null);
      setJob(IDLE);
      setFailed([]);
      setError(null);
    } catch (e) {
      setError((e as Error).message);
    }
  }, []);

  const analyze = useCallback(async () => {
    setError(null);
    setResults(null);
    try {
      const json = await postForm<{ job: BatchJob }>('/api/batch/analyze');
      setJob(json.job);
      startPolling();
    } catch (e) {
      setError((e as Error).message);
    }
  }, [startPolling]);

  return { count, job, results, uploading, error, failed, upload, clear, analyze };
}
