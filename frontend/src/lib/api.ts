/** Thin fetch helpers shared by every hook, so error handling lives in one place. */

export class ApiError extends Error {}

async function parse<T>(res: Response): Promise<T> {
  let json: unknown = null;
  try {
    json = await res.json();
  } catch {
    // non-JSON body; handled below
  }
  if (!res.ok) {
    const msg = (json as { error?: string } | null)?.error;
    throw new ApiError(msg ?? `Request failed (${res.status})`);
  }
  return json as T;
}

export async function postForm<T>(path: string, body?: FormData, signal?: AbortSignal): Promise<T> {
  return parse<T>(await fetch(path, { method: 'POST', body, signal }));
}

export async function getJson<T>(path: string, signal?: AbortSignal): Promise<T> {
  return parse<T>(await fetch(path, { signal }));
}

export async function postBlob(path: string, body: FormData): Promise<Blob> {
  const res = await fetch(path, { method: 'POST', body });
  if (!res.ok) {
    const json = (await res.json().catch(() => null)) as { error?: string } | null;
    throw new ApiError(json?.error ?? `Request failed (${res.status})`);
  }
  return res.blob();
}

export async function fetchBlobUrl(path: string): Promise<string> {
  const res = await fetch(path);
  if (!res.ok) throw new ApiError('Failed to fetch image');
  return URL.createObjectURL(await res.blob());
}

export const isAbort = (e: unknown) => (e as Error | undefined)?.name === 'AbortError';
