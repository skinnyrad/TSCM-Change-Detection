/**
 * Zoom/pan math for ZoomPan. A view is a uniform scale `s` plus a translation
 * (x, y) in container pixels, applied as `translate(x, y) scale(s)` with the
 * transform origin at the top-left. The content always covers the container:
 * no empty space can be panned into view.
 */
export interface View {
  s: number;
  x: number;
  y: number;
}

export const IDENTITY: View = { s: 1, x: 0, y: 0 };
export const MIN_ZOOM = 1;
export const MAX_ZOOM = 10;

const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v));

/** Clamp scale to [MIN_ZOOM, MAX_ZOOM] and translation so the content covers a w×h container. */
export function clampView(v: View, w: number, h: number): View {
  const s = clamp(v.s, MIN_ZOOM, MAX_ZOOM);
  return { s, x: clamp(v.x, w - w * s, 0), y: clamp(v.y, h - h * s, 0) };
}

/** Zoom by `factor` keeping the content point under (px, py) fixed on screen. */
export function zoomAt(v: View, factor: number, px: number, py: number, w: number, h: number): View {
  const s = clamp(v.s * factor, MIN_ZOOM, MAX_ZOOM);
  const k = s / v.s;
  return clampView({ s, x: px - (px - v.x) * k, y: py - (py - v.y) * k }, w, h);
}

export function panBy(v: View, dx: number, dy: number, w: number, h: number): View {
  return clampView({ s: v.s, x: v.x + dx, y: v.y + dy }, w, h);
}

/** The visible part of a w×h content, in fractions (0–1) of its size. */
export function visibleRect(v: View, w: number, h: number): { x: number; y: number; w: number; h: number } {
  return { x: -v.x / (v.s * w), y: -v.y / (v.s * h), w: 1 / v.s, h: 1 / v.s };
}
