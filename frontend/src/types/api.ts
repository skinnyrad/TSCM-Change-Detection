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

// Response from POST /api/analyze (highlight only)
export interface AnalyzeResponse {
  stats: AnalyzeStats;
  images: { highlight?: string };
  before_dims: Dims;
  after_dims: Dims;
  resized: boolean;
}

// Response from POST /api/analyze/diff, /api/analyze/subtraction, /api/analyze/heatmap
export interface AltImageResponse {
  image: string;
}

// Response from POST /api/analyze/canny
export interface CannyResponse {
  edges?: string;
  contours?: string;
}
