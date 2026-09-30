export interface AnchorPoint {
  x: number; // relative [0,1] within image display bounds
  y: number;
}

export interface PointPair {
  id: number;
  src: AnchorPoint | null; // point in "before" image
  dst: AnchorPoint | null; // point in "after" image
}

export interface AnalyzeStats {
  pct: number;
  changed_px: number;
  regions: number;
}

export interface Dims {
  w: number;
  h: number;
}

export interface AutoWarpPairResponse {
  src: [number, number];
  dst: [number, number];
  confidence: number;
}

export interface AutoWarpResponse {
  pairs: AutoWarpPairResponse[];
  confidence: number;
  match_count: number;
  inlier_count: number;
}

export interface Registration {
  mode: 'none' | 'resize' | 'auto' | 'manual';
  applied: boolean;
  inliers: number;
  matches: number;
  confidence: number;
  shift_x: number;
  shift_y: number;
  message: string;
}

/** A detected change region, in fractions (0–1) of the analysed image. */
export interface Region {
  rank: number;
  x: number;
  y: number;
  w: number;
  h: number;
  area_px: number;
  area_pct: number;
  /** Heat mass relative to the strongest region (1 = strongest). */
  score: number;
  /** Mean change magnitude inside the region (0–255). */
  mean_heat: number;
}

// Response from POST /api/analyze
export interface AnalyzeResponse {
  stats: AnalyzeStats;
  regions: Region[];
  images: { highlight?: string };
  before_dims: Dims;
  after_dims: Dims;
  analysis_dims: Dims;
  resized: boolean;
  threshold: number;
  baselines: number;
  registration: Registration;
}

export interface UploadResponse {
  dims: Dims;
  version: number;
  baselines: number;
  registration?: Registration;
}

// Response from POST /api/analyze/alternate
export interface AlternateResponse {
  images: {
    diff?: string;
    subtraction?: string;
    heatmap?: string;
    edges?: string;
    contours?: string;
  };
  threshold: number;
}

/** A rectangle in fractions (0–1) of the image: [x0, y0, x1, y1]. */
export type IgnoreRect = [number, number, number, number];
