import type { IgnoreRect } from '../types/api';

/** Every detection control, shared by the Change Detection, Alternate and Report views. */
export interface DetectionSettings {
  strength: number; // UI slider 5–100; higher = more sensitive
  morphSize: number;
  minRegion: number;
  preBlurSigma: number;
  closeSize: number;
  normalizeLuma: boolean; // fallback when matchIntensity is off
  matchIntensity: boolean; // per-channel exposure/white-balance fit
  colorAware: boolean; // include chroma (catches colour-only changes)
  shiftTolerance: number; // ±px residual misalignment tolerated
  adaptiveThreshold: boolean; // derive threshold from the noise floor
  localLight: boolean; // match lighting locally (shading, lamps, uneven white balance)
  highlightColor: string;
  highlightAlpha: number; // 0–100
  ignore: IgnoreRect[];
}

export const DEFAULT_SETTINGS: DetectionSettings = {
  strength: 75,
  morphSize: 7,
  minRegion: 50,
  preBlurSigma: 2.0,
  closeSize: 5,
  normalizeLuma: true,
  matchIntensity: true,
  colorAware: true,
  shiftTolerance: 0,
  adaptiveThreshold: false,
  localLight: false,
  highlightColor: '#ff3c3c',
  highlightAlpha: 55,
  ignore: [],
};

export const HIGHLIGHT_PRESETS = [
  { label: 'Red', hex: '#ff3c3c' },
  { label: 'Orange', hex: '#ff8c00' },
  { label: 'Yellow', hex: '#ffd700' },
  { label: 'Cyan', hex: '#00e5ff' },
  { label: 'Lime', hex: '#39ff14' },
];

export function hexToRgb(hex: string): [number, number, number] {
  const m = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex);
  if (!m) return [255, 60, 60];
  return [parseInt(m[1] ?? 'ff', 16), parseInt(m[2] ?? '3c', 16), parseInt(m[3] ?? '3c', 16)];
}

/** The server thresholds on change magnitude, so a more sensitive slider means a lower threshold. */
export const strengthToThreshold = (strength: number) => 100 - strength;

/** Build the multipart form the /api/analyze* endpoints expect. */
export function toForm(s: DetectionSettings, withHighlight = false): FormData {
  const fd = new FormData();
  fd.append('strength', String(strengthToThreshold(s.strength)));
  fd.append('auto_threshold', s.adaptiveThreshold ? '1' : '0');
  fd.append('min_region', String(s.minRegion));
  fd.append('morph_size', String(s.morphSize));
  fd.append('close_size', String(s.closeSize));
  fd.append('pre_blur_sigma', String(s.preBlurSigma));
  fd.append('normalize_luma', s.normalizeLuma ? '1' : '0');
  fd.append('match_intensity', s.matchIntensity ? '1' : '0');
  fd.append('color_weight', s.colorAware ? '1' : '0');
  fd.append('shift_tol', String(s.shiftTolerance));
  fd.append('local_light', s.localLight ? '1' : '0');
  if (s.ignore.length > 0) fd.append('ignore', JSON.stringify(s.ignore));
  if (withHighlight) {
    const [r, g, b] = hexToRgb(s.highlightColor);
    fd.append('highlight_r', String(r));
    fd.append('highlight_g', String(g));
    fd.append('highlight_b', String(b));
    fd.append('highlight_alpha', String(s.highlightAlpha / 100));
  }
  return fd;
}
