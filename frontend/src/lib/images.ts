import type { Region } from '../types/api';

export function loadImage(url: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => resolve(img);
    img.onerror = () => reject(new Error('Image failed to load'));
    img.src = url;
  });
}

/** Downscale to at most maxDim on the long edge and encode as JPEG (keeps exported reports small). */
export async function toJpegDataUrl(url: string, maxDim = 1600, quality = 0.88): Promise<string> {
  const img = await loadImage(url);
  const s = Math.min(1, maxDim / Math.max(img.naturalWidth, img.naturalHeight));
  const c = document.createElement('canvas');
  c.width = Math.max(1, Math.round(img.naturalWidth * s));
  c.height = Math.max(1, Math.round(img.naturalHeight * s));
  c.getContext('2d')!.drawImage(img, 0, 0, c.width, c.height);
  return c.toDataURL('image/jpeg', quality);
}

/** Crop a region (plus padding, as a fraction of the image) out of an image. */
export async function cropDataUrl(url: string, r: Pick<Region, 'x' | 'y' | 'w' | 'h'>, pad = 0.15, maxDim = 480): Promise<string> {
  const img = await loadImage(url);
  const x0 = Math.max(0, r.x - r.w * pad), y0 = Math.max(0, r.y - r.h * pad);
  const x1 = Math.min(1, r.x + r.w * (1 + pad)), y1 = Math.min(1, r.y + r.h * (1 + pad));
  const sw = (x1 - x0) * img.naturalWidth, sh = (y1 - y0) * img.naturalHeight;
  const s = Math.min(1, maxDim / Math.max(sw, sh));
  const c = document.createElement('canvas');
  c.width = Math.max(1, Math.round(sw * s));
  c.height = Math.max(1, Math.round(sh * s));
  c.getContext('2d')!.drawImage(img, x0 * img.naturalWidth, y0 * img.naturalHeight, sw, sh, 0, 0, c.width, c.height);
  return c.toDataURL('image/jpeg', 0.9);
}

export async function sha256Hex(file: Blob): Promise<string> {
  const buf = await crypto.subtle.digest('SHA-256', await file.arrayBuffer());
  return [...new Uint8Array(buf)].map(b => b.toString(16).padStart(2, '0')).join('');
}
